package credentials

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cliffom/bambustand/internal/appidentity"
	"github.com/cliffom/bambustand/internal/cloud"
)

func TestCredentialNamespaceMigration(t *testing.T) {
	for _, platform := range []string{"darwin", "linux"} {
		for _, region := range []string{"global", "china"} {
			t.Run(platform+"/"+region, func(t *testing.T) {
				value := cloud.Session{Token: "fixture-token", Username: "synthetic"}
				encoded, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				data := map[string]string{appidentity.LegacyService + region: string(encoded)}
				failSave, failReadback, unavailable, failDelete := false, false, false, false
				store := Store{Region: region, platform: platform}
				store.execute = func(input, name string, args ...string) ([]byte, error) {
					if unavailable {
						return nil, commandError{36}
					}
					joined := strings.Join(args, " ")
					service := ""
					for i, arg := range args {
						if (arg == "-s" || arg == "service") && i+1 < len(args) {
							service = args[i+1]
							break
						}
					}
					saving := strings.Contains(joined, "store") || joined == "-i"
					deleting := strings.Contains(joined, "delete-generic-password") || strings.HasPrefix(joined, "clear ")
					payload := input
					if joined == "-i" {
						parts := strings.SplitN(input, ` -s "`, 2)
						if len(parts) != 2 {
							t.Fatal("no service")
						}
						service = strings.SplitN(parts[1], `"`, 2)[0]
						payload = string(encoded)
					}
					if saving {
						if failSave {
							return nil, commandError{36}
						}
						if !failReadback {
							data[service] = payload
						}
						return nil, nil
					}
					if deleting {
						if failDelete {
							return nil, commandError{36}
						}
						delete(data, service)
						return nil, nil
					}
					if saved, ok := data[service]; ok {
						return []byte(saved), nil
					}
					if platform == "darwin" {
						return nil, commandError{44}
					}
					return nil, commandError{1}
				}
				unavailable = true
				if _, err := store.Load(); !errors.Is(err, ErrUnavailable) {
					t.Fatal(err)
				}
				unavailable = false
				failSave = true
				if _, err := store.Load(); !errors.Is(err, ErrUnavailable) {
					t.Fatal("failed save reported success", err)
				}
				failSave = false
				failReadback = true
				if _, err := store.Load(); !errors.Is(err, ErrUnavailable) {
					t.Fatal("unverified save reported success", err)
				}
				failReadback = false
				got, err := store.Load()
				if err != nil || got != value {
					t.Fatal(got, err)
				}
				if _, ok := data[store.service()]; !ok {
					t.Fatal("no canonical credential")
				}
				// Canonical credentials always win over older data, without a legacy read.
				delete(data, appidentity.LegacyService+region)
				got, err = store.Load()
				if err != nil || got != value {
					t.Fatal(got, err)
				}
				data[appidentity.LegacyService+region] = string(encoded)
				failDelete = true
				if err := store.Delete(); !errors.Is(err, ErrUnavailable) {
					t.Fatal("failed logout reported success", err)
				}
				failDelete = false
				if _, ok := data[store.service()]; !ok {
					t.Fatal("partial logout permits resurrection")
				}
				if err := store.Delete(); err != nil {
					t.Fatal(err)
				}
				if len(data) != 0 {
					t.Fatal("logout retained a saved session")
				}
				if _, err := store.Load(); !errors.Is(err, ErrNotFound) {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCredentialMigrationContention(t *testing.T) {
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	lock, err := appidentity.Lock(filepath.Join(base, appidentity.Namespace, "credentials-global.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	store := Store{Region: "global", execute: func(string, string, ...string) ([]byte, error) {
		t.Fatal("native access while migration locked")
		return nil, nil
	}}
	if _, err := store.Load(); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if err := store.Save(cloud.Session{Token: "fixture-token", Username: "synthetic"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if err := store.Delete(); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestSecretToolDiagnosticIsUnavailableRatherThanMissing(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "secret-tool")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'private service unlock failure' >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	_, err := run("", "secret-tool", "lookup", "service", appidentity.Namespace+".global", "account", "session")
	if !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "private service") {
		t.Fatal("diagnostic misclassified or exposed", err)
	}
}
