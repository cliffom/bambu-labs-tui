package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveNotificationDefaultsPreservesOtherSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	rules := DefaultAlerts().AlertRules
	rules.DesktopNotifications = true
	if err := SaveNotificationDefaults(path, rules); err != nil {
		t.Fatal(err)
	}
	initial := "region = 'china'\nlow_filament_percent = 25\n[alerts]\nautomatic_modals = false\nfilament = false\n[alerts.printers.TEST]\ndesktop_notifications = false\nnotification_sound = true\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	rules.NotificationSound = true
	rules.CompletedPrints = false
	if err := SaveNotificationDefaults(path, rules); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Region != RegionChina || cfg.LowFilamentPercent != 25 || cfg.Alerts.AutomaticModals || cfg.Alerts.Filament {
		t.Fatalf("unrelated settings changed: %+v", cfg)
	}
	if !cfg.Alerts.DesktopNotifications || !cfg.Alerts.NotificationSound || cfg.Alerts.CompletedPrints {
		t.Fatal("notification defaults not saved")
	}
	override := cfg.Alerts.Printers["TEST"]
	if override.DesktopNotifications == nil || *override.DesktopNotifications || override.NotificationSound == nil || !*override.NotificationSound || override.CompletedPrints != nil {
		t.Fatal("override inheritance changed")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("configuration must remain private", err)
	}
	// A malformed/external incompatible edit is never silently overwritten.
	for _, invalid := range []string{"[broken", "unknown_setting = true"} {
		if err := os.WriteFile(path, []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := SaveNotificationDefaults(path, rules); err == nil {
			t.Fatal("overwrote unreadable config")
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != invalid {
			t.Fatal("original config changed", err)
		}
	}
}
