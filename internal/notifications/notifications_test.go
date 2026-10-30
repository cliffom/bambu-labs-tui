package notifications

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestProbeSubmitsSyntheticNotification(t *testing.T) {
	for _, sound := range []bool{false, true} {
		t.Run(map[bool]string{false: "silent", true: "sound"}[sound], func(t *testing.T) {
			calls := 0
			s, helper := fixtureSender(t)
			s.execute = func(ctx context.Context, name string, args ...string) error {
				calls++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > nativeCommandTimeout {
					t.Fatal("native command must have a bounded deadline")
				}
				if name == registrationCommand {
					bundle := filepath.Dir(filepath.Dir(filepath.Dir(helper)))
					if calls != 1 || !slices.Equal(args, []string{"-f", bundle}) {
						t.Fatalf("registration order/path: calls=%d, args=%q", calls, args)
					}
					return nil
				}
				if calls != 2 {
					t.Fatalf("delivery preceded registration: calls=%d", calls)
				}
				want := []string{"--test"}
				if sound {
					want = append(want, "--sound")
				}
				if name != helper || !slices.Equal(args, want) {
					t.Fatalf("unexpected native command: %s %q", name, args)
				}
				return nil
			}
			if err := s.test(context.Background(), sound); err != nil || calls != 2 {
				t.Fatalf("probe result: calls=%d, err=%v", calls, err)
			}
		})
	}
}

func TestProbeUnsupportedEnvironmentsNeverExecute(t *testing.T) {
	for _, tc := range []struct {
		platform string
		remote   bool
		want     string
	}{
		{"linux", false, "macOS only"},
		{"windows", false, "macOS only"},
		{"darwin", true, "SSH is unsupported"},
	} {
		s := sender{platform: tc.platform, remote: tc.remote, execute: func(context.Context, string, ...string) error {
			t.Fatal("unsupported environment attempted delivery")
			return nil
		}}
		if err := s.test(context.Background(), false); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("expected %q, got %v", tc.want, err)
		}
	}
}

func TestProbePreservesNativeFailureAndCancellation(t *testing.T) {
	failure := errors.New("native command failed")
	s, _ := fixtureSender(t)
	s.execute = func(context.Context, string, ...string) error { return failure }
	if err := s.test(context.Background(), false); !errors.Is(err, failure) {
		t.Fatalf("native failure lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.execute = func(ctx context.Context, _ string, _ ...string) error {
		cancel()
		<-ctx.Done()
		return failure
	}
	if err := s.test(ctx, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	s.execute = func(context.Context, string, ...string) error {
		t.Fatal("already canceled request attempted delivery")
		return nil
	}
	if err := s.test(ctx, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request result: %v", err)
	}
}

func TestNativeRunnerPropagatesFailure(t *testing.T) {
	err := run(context.Background(), "/nonexistent/bambustand-test-command")
	if err == nil {
		t.Fatal("missing native command reported success")
	}
}

func fixtureSender(t *testing.T) (sender, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "CLI with spaces")
	helper := filepath.Join(dir, "Bambu Stand Notify.app", "Contents", "MacOS", "bambu-notify")
	if err := os.MkdirAll(filepath.Dir(helper), 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(dir, "bambustand")
	for _, path := range []string{executable, helper} {
		if err := os.WriteFile(path, nil, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	resolved, err := filepath.EvalSymlinks(helper)
	if err != nil {
		t.Fatal(err)
	}
	return sender{platform: "darwin", executable: func() (string, error) { return executable, nil }}, resolved
}

func TestHelperDiscoveryResolvesCLISymlink(t *testing.T) {
	s, helper := fixtureSender(t)
	target, err := s.executable()
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "bambustand-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got, err := helperPath(link)
	// macOS temp roots can themselves be symlinks (/var -> /private/var).
	want, resolveErr := filepath.EvalSymlinks(helper)
	if err != nil || resolveErr != nil || got != want {
		t.Fatalf("helper = %q, want %q; %v, %v", got, want, err, resolveErr)
	}
}

func TestMissingOrInvalidHelperNeverExecutes(t *testing.T) {
	for _, kind := range []string{"missing", "directory", "not executable"} {
		t.Run(kind, func(t *testing.T) {
			s, helper := fixtureSender(t)
			if err := os.Remove(helper); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "directory":
				if err := os.Mkdir(helper, 0o700); err != nil {
					t.Fatal(err)
				}
			case "not executable":
				if err := os.WriteFile(helper, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			s.execute = func(context.Context, string, ...string) error { t.Fatal("invalid helper was executed"); return nil }
			err := s.test(context.Background(), false)
			if err == nil || !strings.Contains(err.Error(), "make notify-helper") {
				t.Fatalf("missing build guidance: %v", err)
			}
		})
	}
}

func TestExecutableDiscoveryFailure(t *testing.T) {
	failure := errors.New("executable unavailable")
	s := sender{platform: "darwin", executable: func() (string, error) { return "", failure }}
	if err := s.test(context.Background(), false); !errors.Is(err, failure) {
		t.Fatalf("discovery failure lost: %v", err)
	}
}

func TestHelperPermissionAndTimeoutFailures(t *testing.T) {
	for _, tc := range []struct{ code, want string }{{"3", "notifications are not allowed"}, {"5", "timed out"}, {"4", "helper failed"}} {
		t.Run(tc.code, func(t *testing.T) {
			t.Setenv("BAMBU_NOTIFY_TEST_EXIT", tc.code)
			nativeErr := exec.Command(os.Args[0], "-test.run=^TestNotificationHelperProcess$").Run()
			if nativeErr == nil {
				t.Fatal("fake helper reported success")
			}
			s, _ := fixtureSender(t)
			s.execute = func(_ context.Context, name string, _ ...string) error {
				if name == registrationCommand {
					return nil
				}
				return nativeErr
			}
			err := s.test(context.Background(), false)
			if !errors.Is(err, nativeErr) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestNotificationHelperProcess(t *testing.T) {
	switch os.Getenv("BAMBU_NOTIFY_TEST_EXIT") {
	case "3":
		os.Exit(3)
	case "4":
		os.Exit(4)
	case "5":
		os.Exit(5)
	}
}

func TestRegistrationFailureSkipsDelivery(t *testing.T) {
	s, _ := fixtureSender(t)
	failure := errors.New("registration failed")
	calls := 0
	s.execute = func(_ context.Context, name string, _ ...string) error {
		calls++
		if name != registrationCommand {
			t.Fatal("delivery attempted after failed registration")
		}
		return failure
	}
	err := s.test(context.Background(), false)
	if calls != 1 || !errors.Is(err, failure) || !strings.Contains(err.Error(), "cannot register") {
		t.Fatalf("registration failure: calls=%d, err=%v", calls, err)
	}
}
