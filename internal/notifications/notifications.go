// Package notifications provides macOS delivery and explicit synthetic probes.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	// The helper waits up to 60 seconds for first-run permission/service access.
	nativeCommandTimeout       = 65 * time.Second
	helperBundleName           = "BambuStand Notify.app"
	helperPermissionDeniedExit = 3
	helperTimeoutExit          = 5
	registrationCommand        = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
)

type sender struct {
	platform   string
	remote     bool
	executable func() (string, error)
	execute    func(context.Context, string, ...string) error
}

// Test submits one synthetic notification through the adjacent branded helper,
// optionally requesting a system sound. Success confirms request acceptance,
// not visible/audible delivery. No account or printer data is read.
func Test(ctx context.Context, sound bool) error {
	s := sender{
		platform:   runtime.GOOS,
		remote:     os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "",
		executable: os.Executable,
		execute:    run,
	}
	return s.test(ctx, sound)
}

func (s sender) test(ctx context.Context, sound bool) error {
	if s.platform != "darwin" {
		return errors.New("desktop notification testing currently supports macOS only; Linux/headless delivery is not implemented")
	}
	if s.remote {
		return errors.New("desktop notification testing over SSH is unsupported; run from a terminal in your local macOS desktop session")
	}
	ctx, cancel := context.WithTimeout(ctx, nativeCommandTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	executable, err := s.executable()
	if err != nil {
		return fmt.Errorf("cannot locate bambustand executable: %w", err)
	}
	helper, err := helperPath(executable)
	if err != nil {
		return fmt.Errorf("branded notification helper unavailable beside bambustand; keep BambuStand Notify.app beside the extracted CLI, or from source run make notify-helper and use ./bin/bambustand: %w", err)
	}
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(helper)))
	if err := s.execute(ctx, registrationCommand, "-f", bundle); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("notification helper registration interrupted: %w", ctx.Err())
		}
		return fmt.Errorf("cannot register the adjacent BambuStand notification helper with macOS: %w", err)
	}
	args := []string{"--test"}
	if sound {
		args = append(args, "--sound")
	}
	if err := s.execute(ctx, helper, args...); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("notification helper interrupted: %w", ctx.Err())
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			switch exit.ExitCode() {
			case helperPermissionDeniedExit:
				return fmt.Errorf("notifications are not allowed; enable BambuStand in System Settings → Notifications and retry: %w", err)
			case helperTimeoutExit:
				return fmt.Errorf("notification helper timed out waiting for macOS permission/service; check the local desktop session and retry: %w", err)
			}
		}
		return fmt.Errorf("branded notification helper failed; check the local desktop session or run the helper directly for details: %w", err)
	}
	return nil
}

func helperPath(executable string) (string, error) {
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}
	helper := filepath.Join(filepath.Dir(resolved), helperBundleName, "Contents", "MacOS", "bambu-notify")
	info, err := os.Stat(helper)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", errors.New("bambu-notify must be a regular executable file inside the adjacent app bundle")
	}
	return helper, nil
}

func run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}
