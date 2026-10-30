package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/bambustand/internal/cloud"
)

// Simulate native stores at the process boundary: each read is a fresh command,
// and arguments are inspected to ensure secrets only travel through stdin.
func fakeStore(t *testing.T, platform string) (Store, *map[string]string) {
	t.Helper()
	data := map[string]string{}
	s := Store{Region: "global", platform: platform}
	s.execute = func(input, name string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "fixture-token") {
			t.Fatal("secret in process arguments")
		}
		if name == "/usr/bin/security" && len(args) == 1 && args[0] == "-i" {
			if strings.Contains(input, "\nquit") {
				t.Fatal("quit masks add command failure")
			}
			parts := strings.SplitN(strings.TrimSuffix(input, "\n"), " -w ", 2)
			if len(parts) != 2 {
				t.Fatal("missing password")
			}
			value, err := strconv.Unquote(parts[1])
			if err != nil {
				t.Fatal(err)
			}
			data["value"] = value
			return nil, nil
		}
		switch {
		case strings.Contains(joined, "store"):
			data["value"] = input
			return nil, nil
		case strings.Contains(joined, "delete-generic-password") || strings.Contains(joined, "clear"):
			delete(data, "value")
			return nil, nil
		default:
			if value, ok := data["value"]; ok {
				return []byte(value + "\n"), nil
			}
			if platform == "darwin" {
				return nil, commandError{44}
			}
			return nil, commandError{1}
		}
	}
	return s, &data
}

func TestNativeStoreLifecycle(t *testing.T) {
	for _, platform := range []string{"darwin", "linux"} {
		t.Run(platform, func(t *testing.T) {
			s, _ := fakeStore(t, platform)
			value := cloud.Session{Token: "fixture-token-with-\"quote\\slash", Username: "u_123"}
			if _, err := s.Load(); !errors.Is(err, ErrNotFound) {
				t.Fatal(err)
			}
			if err := s.Save(value); err != nil {
				t.Fatal(err)
			}
			fresh := s
			got, err := fresh.Load()
			if err != nil || got != value {
				t.Fatal(got, err)
			}
			if err := s.Delete(); err != nil {
				t.Fatal(err)
			}
			if _, err := fresh.Load(); !errors.Is(err, ErrNotFound) {
				t.Fatal("logout left session", err)
			}
			if err := s.Delete(); err != nil {
				t.Fatal("logout is not idempotent", err)
			}
		})
	}
}

func TestUnavailableDoesNotBecomeMissingSession(t *testing.T) {
	for _, code := range []int{36, 152} {
		s := Store{Region: "global", platform: "darwin", execute: func(string, string, ...string) ([]byte, error) { return nil, commandError{code} }}
		if _, err := s.Load(); !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
		if err := s.Delete(); !errors.Is(err, ErrUnavailable) {
			t.Fatal("logout falsely reported success", err)
		}
		if err := s.Save(cloud.Session{Token: "secret", Username: "user"}); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	}
}

func TestErrorsDoNotExposeNativeOutput(t *testing.T) {
	s := Store{Region: "global", platform: "linux", execute: func(string, string, ...string) ([]byte, error) { return nil, errors.New("secret-from-native-stderr") }}
	err := s.Save(cloud.Session{Token: "private-token", Username: "private-user"})
	if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") {
		t.Fatal(err)
	}
}

func TestSaveVerifiesReadback(t *testing.T) {
	s, _ := fakeStore(t, "linux")
	old := s.execute
	s.execute = func(input, name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "lookup" {
			return []byte(`{"token":"other","username":"u_123"}`), nil
		}
		return old(input, name, args...)
	}
	if err := s.Save(cloud.Session{Token: "fixture-token", Username: "u_123"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("unverified write", err)
	}
}

func TestCorruptSessionCanBeDeleted(t *testing.T) {
	s, data := fakeStore(t, "linux")
	(*data)["value"] = "invalid-json"
	if _, err := s.Load(); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestProbeLeavesNoTemporaryCredential(t *testing.T) {
	s, data := fakeStore(t, "darwin")
	s.serviceName = "disposable-probe"
	if err := probe(s); err != nil {
		t.Fatal(err)
	}
	if len(*data) != 0 {
		t.Fatal("probe left temporary credentials")
	}
}

func TestOversizeMacSessionNeverStartsCommand(t *testing.T) {
	s := Store{Region: "global", platform: "darwin", execute: func(string, string, ...string) ([]byte, error) {
		t.Fatal("oversize credential sent to fixed-size command parser")
		return nil, nil
	}}
	if err := s.Save(cloud.Session{Token: strings.Repeat("t", 5000), Username: "u_1"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestSessionValidation(t *testing.T) {
	s, data := fakeStore(t, "linux")
	if err := s.Save(cloud.Session{}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	b, _ := json.Marshal(cloud.Session{Token: "only-token"})
	(*data)["value"] = string(b)
	if _, err := s.Load(); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestLoadContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store, _ := fakeStore(t, "linux")
	store.execute = func(string, string, ...string) ([]byte, error) {
		t.Fatal("native lookup started after cancellation")
		return nil, nil
	}
	cancel()
	if _, err := store.LoadContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestCredentialCommandHelper(t *testing.T) {
	if os.Getenv("BAMBU_CREDENTIAL_WAIT_CHILD") != "1" {
		return
	}
	_, _ = os.Stdout.WriteString("ready")
	time.Sleep(time.Minute)
	os.Exit(0)
}

func TestNativeCommandCancellation(t *testing.T) {
	t.Setenv("BAMBU_CREDENTIAL_WAIT_CHILD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := runContext(ctx, "", os.Args[0], "-test.run=^TestCredentialCommandHelper$"); err == nil {
		t.Fatal("canceled command succeeded")
	}
	if ctx.Err() == nil || time.Since(started) > 3*time.Second {
		t.Fatal("native command did not honor cancellation")
	}
}

func TestUpgradeRetainsCredentialService(t *testing.T) {
	for _, region := range []string{"global", "china"} {
		store := Store{Region: region}
		if got := store.service(); got != "com.github.cliffom.bambu-tui."+region {
			t.Fatalf("saved login namespace changed: %q", got)
		}
	}
}
