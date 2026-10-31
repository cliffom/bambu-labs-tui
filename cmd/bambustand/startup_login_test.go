package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/cliffom/bambustand/internal/cloud"
	"github.com/cliffom/bambustand/internal/credentials"
)

type loginTestStore struct {
	session          cloud.Session
	loadErr, saveErr error
	saves            int
}

func (s *loginTestStore) LoadContext(context.Context) (cloud.Session, error) {
	return s.session, s.loadErr
}

func (s *loginTestStore) Save(session cloud.Session) error {
	s.saves++
	s.session = session
	return s.saveErr
}

func TestMonitorSession(t *testing.T) {
	want := cloud.Session{Token: "synthetic-token", Username: "u_123"}
	for _, tc := range []struct {
		name                       string
		loadErr, loginErr, saveErr error
		interactive, login, save   bool
		feedback                   string
	}{
		{name: "saved", interactive: true},
		{name: "saved noninteractive"},
		{name: "missing interactive", loadErr: credentials.ErrNotFound, interactive: true, login: true, save: true, feedback: "session saved"},
		{name: "missing noninteractive", loadErr: credentials.ErrNotFound, feedback: "run bambustand login --email-code in a terminal"},
		{name: "unavailable", loadErr: credentials.ErrUnavailable, interactive: true, feedback: "unlock the OS credential store"},
		{name: "invalid", loadErr: credentials.ErrInvalid, interactive: true, feedback: "run bambustand login"},
		{name: "cancelled", loadErr: credentials.ErrNotFound, loginErr: errors.New("login cancelled"), interactive: true, login: true, feedback: "login cancelled"},
		{name: "request failed", loadErr: credentials.ErrNotFound, loginErr: cloud.ErrBlocked, interactive: true, login: true, feedback: "request rejected"},
		{name: "save failed", loadErr: credentials.ErrNotFound, saveErr: credentials.ErrUnavailable, interactive: true, login: true, save: true, feedback: "session-only dashboard"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &loginTestStore{session: want, loadErr: tc.loadErr, saveErr: tc.saveErr}
			var output, diagnostics bytes.Buffer
			called := false
			got, err := monitorSession(context.Background(), store, tc.interactive, func(context.Context) (cloud.Session, error) {
				called = true
				return want, tc.loginErr
			}, &output, &diagnostics)
			if called != tc.login || (store.saves == 1) != tc.save {
				t.Fatalf("login=%v saves=%d", called, store.saves)
			}
			success := tc.loadErr == nil || tc.login && tc.loginErr == nil
			if success && (err != nil || got != want) {
				t.Fatalf("session=%v err=%v", got, err)
			}
			if !success && (err == nil || got != (cloud.Session{})) {
				t.Fatalf("session=%v err=%v", got, err)
			}
			feedback := output.String() + diagnostics.String()
			if err != nil {
				feedback += err.Error()
			}
			if !strings.Contains(feedback, tc.feedback) {
				t.Fatalf("missing %q: %s", tc.feedback, feedback)
			}
			if strings.Contains(feedback, want.Token) {
				t.Fatal("token leaked")
			}
			if tc.saveErr != nil && strings.Contains(output.String(), "session saved") {
				t.Fatal("save failure claimed success")
			}
		})
	}
}

func TestMonitorSessionCancelledLookup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &loginTestStore{loadErr: context.Canceled}
	_, err := monitorSession(ctx, store, true, func(context.Context) (cloud.Session, error) {
		t.Fatal("login after cancellation")
		return cloud.Session{}, nil
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || err.Error() != "login cancelled" || store.saves != 0 {
		t.Fatalf("err=%v saves=%d", err, store.saves)
	}
}

func TestLoginPromptMinimumWidthAndMasking(t *testing.T) {
	for _, label := range []string{"Bambu account email", "Email verification code"} {
		input := textinput.New()
		input.Focus()
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '•'
		input.SetValue(strings.Repeat("9", 100))
		model := promptModel{label: label, input: input}
		next, _ := model.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
		view := next.(promptModel).View()
		if ansi.StringWidth(strings.TrimSuffix(view, "\n")) > 60 || strings.Contains(view, "999") {
			t.Fatalf("prompt overflow or exposed secret: %q", view)
		}
	}
}
