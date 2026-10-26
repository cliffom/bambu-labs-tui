package config

import (
	"bytes"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/cliffom/bambu-tui/internal/atomicfile"
)

// SaveNotificationDefaults reloads the file so unrelated settings and printer
// overrides survive. Encoding normalizes TOML formatting and removes comments.
// Callers serialize writes and run this outside the UI reducer.
func SaveNotificationDefaults(path string, rules AlertRules) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	cfg.Alerts.DesktopNotifications = rules.DesktopNotifications
	cfg.Alerts.NotificationSound = rules.NotificationSound
	cfg.Alerts.CompletedPrints = rules.CompletedPrints
	var data bytes.Buffer
	if err := toml.NewEncoder(&data).Encode(cfg); err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	return atomicfile.Replace(path, ".config-*", data.Bytes())
}
