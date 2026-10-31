// Package credentials uses the host's native credential-store command. Secrets
// travel through stdin, never shell interpolation, command arguments or files.
package credentials

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cliffom/bambustand/internal/appidentity"
	"github.com/cliffom/bambustand/internal/cloud"
)

var (
	ErrUnavailable = errors.New("OS credential store unavailable")
	ErrNotFound    = errors.New("no saved account session")
	ErrInvalid     = errors.New("saved account session is invalid; run bambustand login")
)

type (
	command func(input, name string, args ...string) ([]byte, error)

	Store struct {
		Region      string
		platform    string
		execute     command
		serviceName string
	}

	commandError struct{ code int }
)

func (s Store) service() string {
	if s.serviceName != "" {
		return s.serviceName
	}
	return servicePrefix + s.Region
}

func (s Store) os() string {
	if s.platform != "" {
		return s.platform
	}
	return runtime.GOOS
}

func (s Store) run(input, name string, args ...string) ([]byte, error) {
	if s.execute != nil {
		return s.execute(input, name, args...)
	}
	return run(input, name, args...)
}

func (s Store) runContext(ctx context.Context, input, name string, args ...string) ([]byte, error) {
	if s.execute != nil {
		return s.execute(input, name, args...)
	}
	return runContext(ctx, input, name, args...)
}

func (e commandError) Error() string {
	return fmt.Sprintf("credential command exited with status %d", e.code)
}

func run(input, name string, args ...string) ([]byte, error) {
	return runContext(context.Background(), input, name, args...)
}

func runContext(ctx context.Context, input, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, nativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			// secret-tool uses exit 1 for both a missing item and service/unlock
			// failures. A diagnostic on stderr must never trigger legacy fallback or
			// automatic first-launch login. Keep its private text out of errors.
			if name == "secret-tool" && exit.ExitCode() == secretToolNotFoundExit && len(bytes.TrimSpace(stderr.Bytes())) != 0 {
				return nil, ErrUnavailable
			}
			return nil, commandError{exit.ExitCode()}
		}
		return nil, ErrUnavailable
	}
	return stdout.Bytes(), nil
}

func failure(operation string, err error) error {
	var exit commandError
	if errors.As(err, &exit) {
		return fmt.Errorf("%w: %s (native command exit %d)", ErrUnavailable, operation, exit.code)
	}
	return fmt.Errorf("%w: %s", ErrUnavailable, operation)
}

func (s Store) Load() (cloud.Session, error) { return s.LoadContext(context.Background()) }

// LoadContext bounds native-store access and lets foreground hosts cancel startup.
func (s Store) LoadContext(ctx context.Context) (cloud.Session, error) {
	lock, err := s.lock()
	if err != nil {
		return cloud.Session{}, err
	}
	if lock != nil {
		defer func() { _ = lock.Close() }()
	}
	session, err := s.loadContext(ctx)
	if !errors.Is(err, ErrNotFound) || s.serviceName != "" {
		return session, err
	}
	legacy := s
	legacy.serviceName = appidentity.LegacyService + s.Region
	session, err = legacy.loadContext(ctx)
	if err != nil {
		return cloud.Session{}, err
	}
	// Save verifies readback from the canonical service before migration succeeds.
	// Leave the source intact for recovery; logout explicitly deletes both entries.
	if err := s.saveContext(ctx, session); err != nil {
		return cloud.Session{}, err
	}
	return session, nil
}

func (s Store) loadContext(ctx context.Context) (cloud.Session, error) {
	if err := ctx.Err(); err != nil {
		return cloud.Session{}, err
	}
	run := func(input, name string, args ...string) ([]byte, error) {
		if s.execute != nil {
			return s.execute(input, name, args...)
		}
		return runContext(ctx, input, name, args...)
	}
	var data []byte
	var err error
	switch s.os() {
	case "darwin":
		data, err = run("", "/usr/bin/security", "find-generic-password", "-s", s.service(), "-a", "session", "-w")
	case "linux":
		data, err = run("", "secret-tool", "lookup", "service", s.service(), "account", "session")
	default:
		return cloud.Session{}, ErrUnavailable
	}
	if ctx.Err() != nil {
		return cloud.Session{}, ctx.Err()
	}
	if err != nil {
		var exit commandError
		if errors.As(err, &exit) && ((s.os() == "darwin" && exit.code == keychainNotFoundExit) || (s.os() == "linux" && exit.code == secretToolNotFoundExit)) {
			return cloud.Session{}, ErrNotFound
		}
		return cloud.Session{}, failure("cannot read saved session", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return cloud.Session{}, ErrNotFound
	}
	var session cloud.Session
	if json.Unmarshal(bytes.TrimSpace(data), &session) != nil || session.Token == "" || session.Username == "" {
		return cloud.Session{}, ErrInvalid
	}
	return session, nil
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`).Replace(s) + `"`
}

func (s Store) Save(session cloud.Session) error {
	lock, err := s.lock()
	if err != nil {
		return err
	}
	if lock != nil {
		defer func() { _ = lock.Close() }()
	}
	return s.saveContext(context.Background(), session)
}

func (s Store) saveContext(ctx context.Context, session cloud.Session) error {
	if session.Token == "" || session.Username == "" {
		return ErrInvalid
	}
	b, err := json.Marshal(session)
	if err != nil {
		return ErrInvalid
	}
	switch s.os() {
	case "darwin":
		input := "add-generic-password -U -s " + quote(s.service()) + " -a session -w " + quote(string(b)) + "\n"
		// Apple's security -i has a 4096-byte line buffer. EOF preserves the
		// add command's exit code; a trailing quit would erase its failure.
		if len(input) > keychainMaxInputBytes {
			return fmt.Errorf("%w: session exceeds macOS security input limit; use --session", ErrUnavailable)
		}
		_, err = s.runContext(ctx, input, "/usr/bin/security", "-i")
	case "linux":
		_, err = s.runContext(ctx, string(b), "secret-tool", "store", "--label=BambuStand", "service", s.service(), "account", "session")
	default:
		return ErrUnavailable
	}
	if err != nil {
		return failure("cannot save session", err)
	}
	loaded, err := s.loadContext(ctx)
	if err != nil {
		return failure("saved session could not be read back", err)
	}
	if loaded != session {
		return fmt.Errorf("%w: saved session did not match", ErrUnavailable)
	}
	return nil
}

func (s Store) lock() (*os.File, error) {
	if s.serviceName != "" {
		return nil, nil
	}
	if s.Region != "global" && s.Region != "china" {
		return nil, ErrInvalid
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, ErrUnavailable
	}
	f, err := appidentity.Lock(filepath.Join(base, appidentity.Namespace, "credentials-"+s.Region+".lock"))
	if err != nil {
		return nil, fmt.Errorf("%w: saved-session migration/access is busy or unavailable; stop other login/logout commands and retry", ErrUnavailable)
	}
	return f, nil
}

func (s Store) Delete() error {
	lock, err := s.lock()
	if err != nil {
		return err
	}
	if lock != nil {
		defer func() { _ = lock.Close() }()
	}

	// Delete legacy first: partial failure cannot resurrect a removed canonical
	// session through fallback migration on the next launch.
	if s.serviceName == "" {
		legacy := s
		legacy.serviceName = appidentity.LegacyService + s.Region
		if err := legacy.Delete(); err != nil {
			return err
		}
	}
	if _, err := s.loadContext(context.Background()); errors.Is(err, ErrNotFound) {
		return nil
	} else if err != nil && !errors.Is(err, ErrInvalid) {
		return err
	}
	switch s.os() {
	case "darwin":
		_, err = s.run("", "/usr/bin/security", "delete-generic-password", "-s", s.service(), "-a", "session")
	case "linux":
		_, err = s.run("", "secret-tool", "clear", "service", s.service(), "account", "session")
	default:
		return ErrUnavailable
	}
	if err != nil {
		return failure("cannot delete saved session", err)
	}
	return nil
}

// Probe uses a disposable service name and never accesses Bambu credentials.
func Probe(region string) error {
	id := make([]byte, probeRandomBytes)
	if _, err := rand.Read(id); err != nil {
		return errors.New("cannot create credential probe")
	}
	s := Store{Region: region, serviceName: appidentity.Namespace + ".probe." + hex.EncodeToString(id)}
	return probe(s)
}

func probe(s Store) (err error) {
	defer func() {
		cleanup := s.Delete()
		if cleanup != nil && err == nil {
			err = cleanup
		}
	}()
	value := cloud.Session{Token: "disposable-credential-probe", Username: "probe"}
	if err := s.Save(value); err != nil {
		return err
	}
	// A fresh Store performs a separate invocation, like an application restart.
	fresh := s
	got, err := fresh.Load()
	if err != nil {
		return err
	}
	if got != value {
		return ErrInvalid
	}
	return s.Delete()
}
