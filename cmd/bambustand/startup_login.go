package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cliffom/bambustand/internal/cloud"
	"github.com/cliffom/bambustand/internal/credentials"
)

type sessionStore interface {
	LoadContext(context.Context) (cloud.Session, error)
	Save(cloud.Session) error
}

// Only confirmed absence permits automatic login. Failed lookups must not
// replace an existing session or imply that a locked store is empty.
func monitorSession(ctx context.Context, store sessionStore, interactive bool, login func(context.Context) (cloud.Session, error), output, diagnostics io.Writer) (cloud.Session, error) {
	session, err := store.LoadContext(ctx)
	if err == nil {
		return session, nil
	}
	if ctx.Err() != nil {
		return cloud.Session{}, errors.New("login cancelled")
	}
	if !errors.Is(err, credentials.ErrNotFound) {
		return cloud.Session{}, fmt.Errorf("%w; unlock the OS credential store (Linux: install secret-tool and start Secret Service); run bambustand doctor to check storage, or bambustand login --email-code --session to monitor without saving", err)
	}
	if !interactive {
		return cloud.Session{}, fmt.Errorf("%w for this config's region; run bambustand login --email-code in a terminal with the same --config, then retry (or use bambustand --demo)", err)
	}
	_, _ = fmt.Fprintln(output, "No saved Bambu session for this region. Sign in with an emailed verification code (Esc or Ctrl+C cancels).")
	session, err = login(ctx)
	if err != nil {
		return cloud.Session{}, fmt.Errorf("email-code login failed: %w", err)
	}
	if saveLoginSession(store, session, diagnostics) {
		_, _ = fmt.Fprintln(output, "Bambu session saved in the OS credential store. Opening the dashboard.")
	}
	return session, nil
}

// Both explicit and first-launch login retain the validated in-memory session
// on save failure. Store.Save verifies persistence before reporting success.
func saveLoginSession(store sessionStore, session cloud.Session, diagnostics io.Writer) bool {
	if err := store.Save(session); err != nil {
		_, _ = fmt.Fprintf(diagnostics, "Could not save session: %v. Run bambustand doctor for a credential-store check.\n", err)
		_, _ = fmt.Fprintln(diagnostics, "Opening a session-only dashboard; the token was not confirmed saved. Log in again next launch if no usable session was saved.")
		return false
	}
	return true
}
