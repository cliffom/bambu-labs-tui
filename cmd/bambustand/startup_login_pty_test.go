//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/cliffom/bambustand/internal/cloud"
	"github.com/cliffom/bambustand/internal/config"
	"github.com/cliffom/bambustand/internal/credentials"
	"github.com/cliffom/bambustand/internal/ui"
)

// Real prompts and cloud authentication against synthetic HTTP; the validated
// account hands off to the actual dashboard with demo telemetry and no keychain.
func runLoginPTYFixture(t *testing.T, mode string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/user-service/user/sendemail/code":
			if mode == "login-request-failure" {
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `{"private":"synthetic-token 741963"}`)
				return
			}
			_, _ = io.WriteString(w, `{"code":0}`)
		case "/v1/user-service/user/login":
			var body struct {
				Account, Code string
				Password      *string
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Account != "synthetic@example.invalid" || body.Code != "741963" || body.Password != nil {
				t.Error("email-code request not used")
			}
			if mode == "login-rejected" {
				_, _ = io.WriteString(w, `{"code":1,"accessToken":"synthetic-token","message":"741963"}`)
				return
			}
			_, _ = io.WriteString(w, `{"accessToken":"synthetic-token"}`)
		case "/v1/design-user-service/my/preference":
			if r.Header.Get("Authorization") != "Bearer synthetic-token" {
				t.Error("validation missing token")
			}
			if mode == "login-account-failure" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = io.WriteString(w, `{"uid":123}`)
		default:
			t.Error("unexpected authentication request")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	api := cloud.New(config.RegionGlobal)
	api.BaseURL = server.URL
	store := &loginTestStore{loadErr: credentials.ErrNotFound}
	if mode == "login-existing" {
		store.loadErr = nil
		store.session = cloud.Session{Token: "synthetic-token", Username: "u_123"}
	}
	if mode == "login-unavailable" {
		store.loadErr = credentials.ErrUnavailable
	}
	if mode == "login-save-failure" {
		store.saveErr = credentials.ErrUnavailable
	}
	account, err := monitorSession(ctx, store, term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()), func(ctx context.Context) (cloud.Session, error) { return authenticate(ctx, api, false, true) }, os.Stdout, os.Stderr)
	if err != nil {
		if store.saves != 0 {
			t.Error("failed authentication saved")
		}
		return err
	}
	if account != (cloud.Session{Token: "synthetic-token", Username: "u_123"}) {
		t.Fatal("dashboard did not receive validated session")
	}
	cfg, err := config.Load(os.Getenv("BAMBU_STAND_PTY_CONFIG"))
	if err != nil {
		return err
	}
	return dashboard(ctx, os.Getenv("BAMBU_STAND_PTY_CONFIG"), cfg, ui.Theme{}, nil, cloud.Session{}, true)
}

func TestPTYStartupLoginContinues(t *testing.T) {
	for _, mode := range []string{"login-success", "login-save-failure", "login-existing"} {
		t.Run(mode, func(t *testing.T) {
			child := startPTYChild(t, mode)
			if mode != "login-existing" {
				child.expect(t, 0, "Bambu account email")
				child.resize(t, 60, 18)
				child.send(t, "synthetic@example.invalid\r")
				child.expect(t, 0, "Email verification code")
				child.send(t, "741")
				child.expect(t, 0, "•••")
				output, _ := child.transcript()
				t.Logf("60 x 18 login prompt transcript:\n%s", ansi.Strip(string(output)))
				child.send(t, "963\r")
			}
			if mode == "login-success" {
				child.expect(t, 0, "session saved in the OS credential store")
			}
			if mode == "login-save-failure" {
				child.expect(t, 0, "token was not confirmed saved")
			}
			child.expect(t, 0, "/ DEMO CONNECTED / FARM")
			child.expect(t, 0, "q quit")
			frame, _ := child.transcript()
			t.Logf("Dashboard continuation transcript:\n%s", ansi.Strip(string(frame)))
			child.send(t, "q")
			child.exited(t, 0)
			output, _ := child.transcript()
			for _, secret := range []string{"synthetic-token", "741963"} {
				if strings.Contains(string(output), secret) {
					t.Fatal("secret leaked")
				}
			}
			if mode == "login-existing" && strings.Contains(string(output), "Bambu account email") {
				t.Fatal("prompted with saved credentials")
			}
		})
	}
}

func TestPTYStartupLoginFailure(t *testing.T) {
	for _, mode := range []string{"login-unavailable", "login-rejected", "login-request-failure", "login-account-failure"} {
		t.Run(mode, func(t *testing.T) {
			child := startPTYChild(t, mode)
			if mode != "login-unavailable" {
				child.expect(t, 0, "Bambu account email")
				child.send(t, "synthetic@example.invalid\r")
				if mode != "login-request-failure" {
					child.expect(t, 0, "Email verification code")
					child.send(t, "741963\r")
				}
			}
			feedback := map[string]string{"login-unavailable": "unlock the OS credential store", "login-rejected": "verification code rejected", "login-request-failure": "request rejected by Bambu Cloud", "login-account-failure": "session expired or rejected"}[mode]
			child.expect(t, 0, feedback)
			child.exited(t, 1)
			output, _ := child.transcript()
			for _, secret := range []string{"synthetic-token", "741963", "\x1b[?1049h"} {
				if strings.Contains(string(output), secret) {
					t.Fatal("secret leaked or dashboard entered after failure")
				}
			}
		})
	}
}

func TestPTYStartupLoginCancellation(t *testing.T) {
	for _, stage := range []string{"email", "code"} {
		for _, key := range []string{"\x1b", "\x03"} {
			t.Run(stage+key, func(t *testing.T) {
				child := startPTYChild(t, "login-cancel")
				child.expect(t, 0, "Bambu account email")
				if stage == "code" {
					child.send(t, "synthetic@example.invalid\r")
					child.expect(t, 0, "Email verification code")
				}
				child.send(t, key)
				child.expect(t, 0, "login cancelled")
				child.exited(t, 1)
			})
		}
	}
}
