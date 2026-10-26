package config

import (
	"bytes"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/cliffom/bambu-tui/internal/atomicfile"
)

// SaveChanges reloads valid TOML and applies only fields edited since the last
// successful snapshot. Unrelated external edits survive; edited fields win.
// Callers serialize writes outside the UI reducer. Encoding removes comments.
func SaveChanges(path string, before, after Config) error {
	before.View, after.View = before.View.Normalized(), after.View.Normalized()
	if err := after.Validate(); err != nil {
		return err
	}
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	if before.Region != after.Region {
		cfg.Region = after.Region
	}
	if before.LowFilamentPercent != after.LowFilamentPercent {
		cfg.LowFilamentPercent = after.LowFilamentPercent
	}
	if before.View.Filter != after.View.Filter {
		cfg.View.Filter = after.View.Filter
	}
	if before.View.Sort != after.View.Sort {
		cfg.View.Sort = after.View.Sort
	}
	for field := range AlertFieldCount {
		if before.Alerts.Value(field) != after.Alerts.Value(field) {
			cfg.Alerts.Set(field, after.Alerts.Value(field))
		}
	}
	if before.Alerts.QuietHours != after.Alerts.QuietHours {
		cfg.Alerts.QuietHours = after.Alerts.QuietHours
	}
	if before.Alerts.BurstWindowSeconds != after.Alerts.BurstWindowSeconds {
		cfg.Alerts.BurstWindowSeconds = after.Alerts.BurstWindowSeconds
	}
	ids := map[string]bool{}
	for id := range before.Alerts.Printers {
		ids[id] = true
	}
	for id := range after.Alerts.Printers {
		ids[id] = true
	}
	for id := range ids {
		old, next, current := before.Alerts.Printers[id], after.Alerts.Printers[id], cfg.Alerts.Printers[id]
		for field := range AlertFieldCount {
			a, b := old.Value(field), next.Value(field)
			if (a == nil) != (b == nil) || (a != nil && b != nil && *a != *b) {
				current.Set(field, b)
			}
		}
		empty := true
		for field := range AlertFieldCount {
			empty = empty && current.Value(field) == nil
		}
		if empty {
			delete(cfg.Alerts.Printers, id)
		} else {
			if cfg.Alerts.Printers == nil {
				cfg.Alerts.Printers = map[string]AlertOverride{}
			}
			cfg.Alerts.Printers[id] = current
		}
	}
	var data bytes.Buffer
	if err := toml.NewEncoder(&data).Encode(cfg); err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	return atomicfile.Replace(path, ".config-*", data.Bytes())
}
