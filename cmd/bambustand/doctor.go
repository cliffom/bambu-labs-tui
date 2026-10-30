package main

import (
	"context"
	"fmt"
	"io"

	"github.com/cliffom/bambustand/internal/buildinfo"
)

func notificationDoctor(ctx context.Context, out io.Writer, sound bool, send func(context.Context, bool) error) error {
	if _, err := fmt.Fprintf(out, "%s\nSubmitting a synthetic desktop notification through the branded BambuStand helper (sound requested: %t)…\nAllow BambuStand notifications if macOS prompts; the helper waits up to 60 seconds.\n", buildinfo.String(), sound); err != nil {
		return err
	}
	if err := send(ctx, sound); err != nil {
		return fmt.Errorf("notification test failed: %w", err)
	}
	_, err := fmt.Fprintln(out, "macOS accepted the branded notification request. Banner and sound delivery are not confirmed.\nCheck your desktop and Notification Center. If absent, check BambuStand notification permissions, Show previews, and Focus settings.\nNo Bambu credentials or printer data were used. This test does not enable automatic notifications.")
	return err
}
