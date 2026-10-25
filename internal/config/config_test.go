package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaultsAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	c, err := Load(path)
	if err != nil || c.Region != "global" || c.LowFilamentPercent != 10 {
		t.Fatal(c, err)
	}
	for _, text := range []string{`region = "invalid"`, `low_filament_percent = 101`, `low_filament_percent = -1`, `token = "must-not-be-configured"`, `region = 3`} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted %s", text)
		}
	}
	if err := os.WriteFile(path, []byte("region = \"china\"\nlow_filament_percent = 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(path)
	if err != nil || c.Region != "china" || c.LowFilamentPercent != 0 {
		t.Fatal(c, err)
	}
}
