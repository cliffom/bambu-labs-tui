package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cliffom/bambu-tui/internal/accountstore"
	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/credentials"
	"github.com/cliffom/bambu-tui/internal/notification_state"
)

// This explicit maintenance operation never runs alongside a cloud host. It
// consumes unfinished completion tracking, but never resets delivery identities.
func compactNotifications(ctx context.Context, cfg config.Config, output io.Writer) error {
	account, err := (credentials.Store{Region: cfg.Region}).LoadContext(ctx)
	if err != nil {
		return errors.New("cannot load saved account; unlock credentials or run login before compact-notifications")
	}
	return compactAccountNotifications(cfg.Region, account.Username, output)
}

func compactAccountNotifications(region, username string, output io.Writer) error {
	stores, err := accountstore.Open(region, username)
	if err != nil {
		return err
	}
	defer stores.Close()
	ledger, err := notification_state.New(region, username)
	if err != nil {
		return err
	}
	if err := ledger.CompactJobs(); err != nil {
		return err
	}
	ledger.Path += ".browser"
	if err := ledger.CompactJobs(); err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "Native and browser job histories compacted without forgetting identities. Retained unfinished jobs will not notify completion; new job identities can notify. Intervention history and saved notice decisions are unchanged. Restart monitoring to resume delivery.")
	return err
}
