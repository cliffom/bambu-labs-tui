package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveChangesMergesOnlyEditedSettingsAndProtectsInvalidFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	before, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.Alerts.DesktopNotifications = true
	if err := SaveChanges(path, before, after); err != nil {
		t.Fatal(err)
	}
	before = after.Clone()
	// Concurrent external edits to untouched fields must survive subsequent UI saves.
	external := "region = 'china'\nlow_filament_percent = 25\n[alerts]\ndesktop_notifications = true\nautomatic_modals = false\nfilament = false\n[alerts.printers.ABSENT]\ndesktop_notifications = false\nnotification_sound = true\n"
	if err := os.WriteFile(path, []byte(external), 0o600); err != nil {
		t.Fatal(err)
	}
	after = before.Clone()
	after.Alerts.NotificationSound = true
	after.Alerts.CompletedPrints = false
	if err := SaveChanges(path, before, after); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Region != RegionChina || cfg.LowFilamentPercent != 25 || cfg.Alerts.AutomaticModals || cfg.Alerts.Filament {
		t.Fatalf("untouched external settings changed: %+v", cfg)
	}
	if !cfg.Alerts.DesktopNotifications || !cfg.Alerts.NotificationSound || cfg.Alerts.CompletedPrints {
		t.Fatal("edited settings not saved")
	}
	override := cfg.Alerts.Printers["ABSENT"]
	if override.DesktopNotifications == nil || *override.DesktopNotifications || override.NotificationSound == nil || !*override.NotificationSound || override.CompletedPrints != nil {
		t.Fatal("override inheritance changed")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("configuration must remain private", err)
	}
	for _, invalid := range []string{"[broken", "unknown_setting = true"} {
		if err := os.WriteFile(path, []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := SaveChanges(path, before, after); err == nil {
			t.Fatal("overwrote unreadable config")
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != invalid {
			t.Fatal("original config changed", err)
		}
	}
}

func TestEveryAlertFieldRoundTripsAndClonesIndependently(t *testing.T) {
	if int(AlertFieldCount) != reflect.TypeFor[AlertRules]().NumField() || int(AlertFieldCount) != reflect.TypeFor[AlertOverride]().NumField() {
		t.Fatal("editing catalog omits supported fields")
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	baseline, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	next := baseline.Clone()
	next.Region, next.LowFilamentPercent = RegionChina, 0
	next.Alerts.Printers = map[string]AlertOverride{}
	var override AlertOverride
	for field := range AlertFieldCount {
		value := !baseline.Alerts.Value(field)
		next.Alerts.Set(field, value)
		override.Set(field, &value)
	}
	next.Alerts.Printers["ABSENT"] = override
	snapshot := next.Clone()
	override = next.Alerts.Printers["ABSENT"]
	*override.Value(DesktopNotifications) = !*override.Value(DesktopNotifications)
	if reflect.DeepEqual(snapshot, next) {
		t.Fatal("snapshot aliases mutable override")
	}
	next = snapshot
	if err := SaveChanges(path, baseline, next); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || !reflect.DeepEqual(loaded, next) {
		t.Fatalf("all-field roundtrip: %+v %v", loaded, err)
	}
	reset := next.Clone()
	delete(reset.Alerts.Printers, "ABSENT")
	if err := SaveChanges(path, next, reset); err != nil {
		t.Fatal(err)
	}
	loaded, err = Load(path)
	if err != nil || len(loaded.Alerts.Printers) != 0 {
		t.Fatal("reset did not remove overrides", err)
	}
	// Reject invalid interactive snapshots before touching the file.
	for _, invalid := range []Config{{Region: "elsewhere", Alerts: DefaultAlerts()}, {Region: RegionGlobal, LowFilamentPercent: 101, Alerts: DefaultAlerts()}} {
		if err := SaveChanges(path, reset, invalid); err == nil {
			t.Fatal("invalid snapshot saved")
		}
	}
}

func TestScheduleRoundTripAndExternalScheduleSurvivesUIEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.Alerts.QuietHours = QuietHours{Enabled: true, Timezone: "America/Los_Angeles", Start: "22:00", End: "07:00", Days: "mon,tue,wed,thu,fri"}
	after.Alerts.BurstWindowSeconds = 3
	if err := SaveChanges(path, before, after); err != nil {
		t.Fatal(err)
	}
	// The UI baseline predates the external schedule edit.
	edited := before.Clone()
	edited.Alerts.NotificationSound = true
	if err := SaveChanges(path, before, edited); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Alerts.QuietHours != after.Alerts.QuietHours || loaded.Alerts.BurstWindowSeconds != 3 || !loaded.Alerts.NotificationSound {
		t.Fatal(loaded)
	}
	for _, seconds := range []int{-1, 61} {
		loaded.Alerts.BurstWindowSeconds = seconds
		if loaded.Validate() == nil {
			t.Fatal("accepted invalid window", seconds)
		}
	}
}
