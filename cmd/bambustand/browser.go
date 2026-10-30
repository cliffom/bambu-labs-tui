package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/cliffom/bambustand/internal/notification_state"
	"github.com/cliffom/bambustand/internal/webserver"
)

// Browser delivery uses its own account ledger on every desktop platform. Demo
// gets temporary isolated storage; native permission/helper availability is unrelated.
func browserServer(ctx context.Context, region, username string, demo bool) (*webserver.Server, func()) {
	var ledger webserver.BrowserLedger
	cleanup := func() {}
	if demo {
		if dir, err := os.MkdirTemp("", "bambustand-demo-browser-*"); err == nil {
			ledger = notification_state.Store{Path: filepath.Join(dir, "state.json")}
			cleanup = func() { _ = os.RemoveAll(dir) }
		}
	} else if store, err := notification_state.New(region, username); err == nil {
		store.Path += ".browser"
		ledger = store
	}
	server := webserver.New(ctx, ledger)
	return server, func() {
		server.Close()
		select {
		case <-server.NotificationsDone():
			cleanup()
		case <-time.After(serveShutdownTimeout):
			// Keep demo storage if an I/O call has not returned; never remove/recreate
			// a ledger underneath a surviving worker.
		}
	}
}
