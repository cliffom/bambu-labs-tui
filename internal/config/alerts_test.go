package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAlertDefaultsAndOverrides checks TOML inheritance including explicit false values.
func TestAlertDefaultsAndOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[alerts]
automatic_modals = false
filament = false
[alerts.printers."H"]
automatic_modals = true
extrusion = false
[alerts.printers."X"]
filament = true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	h, x, other := c.Alerts.RulesFor("H"), c.Alerts.RulesFor("X"), c.Alerts.RulesFor("other")
	if !h.AutomaticModals || h.Filament || h.Extrusion || !h.FailedPrints || !h.PrinterFaults || !h.UnknownPauses {
		t.Fatalf("independent inheritance failed: %+v", h)
	}
	if x.AutomaticModals || !x.Filament || !x.Extrusion || other.AutomaticModals || other.Filament {
		t.Fatal("override leaked across printers or ignored the global default")
	}
	if h.Allows([]string{"filament", "extrusion"}) || !h.Allows([]string{"filament", "failed_prints"}) || x.Allows([]string{"filament"}) || h.Allows([]string{"unknown"}) {
		t.Fatal("category/automatic-modal policy was not applied")
	}
	if err := os.WriteFile(path, []byte("region = \"china\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(path)
	if err != nil || !c.Alerts.RulesFor("any").Allows([]string{"filament"}) {
		t.Fatal("old configurations changed alert defaults")
	}
}

// TestInvalidAlertSettings rejects unknown fields, wrong types, and empty printer IDs.
func TestInvalidAlertSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, text := range []string{
		"[alerts]\nsound = true", "[alerts]\nfilament = 'off'", "[alerts]\nunknown = false",
		"[alerts.printers.H]\nextrusion = 2", "[alerts.printers.H]\nsounds = true", "[alerts.printers.\"\"]\nfilament = false",
	} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted invalid alert config: %s", text)
		}
	}
}

// TestExampleConfigurationKeepsAlertDefaults ensures the shipped example is directly usable.
func TestExampleConfigurationKeepsAlertDefaults(t *testing.T) {
	c, err := Load("../../config.example.toml")
	if err != nil || !c.Alerts.RulesFor("any").AnyCategoryEnabled() || len(c.Alerts.Printers) != 0 {
		t.Fatalf("invalid example: %+v, %v", c, err)
	}
}
