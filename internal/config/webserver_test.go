package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWebServerDefaultAndMergedPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.WebServer.Enabled {
		t.Fatal("web server must default off")
	}
	if err := os.WriteFile(path, []byte("region = \"china\"\nlow_filament_percent = 25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.WebServer.Enabled = true
	if err := SaveChanges(path, before, after); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.WebServer.Enabled || got.Region != RegionChina || got.LowFilamentPercent != 25 {
		t.Fatal("web edit lost unrelated external preferences", got)
	}
	after = got.Clone()
	after.WebServer.Enabled = false
	if err := SaveChanges(path, got, after); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path)
	if err != nil || got.WebServer.Enabled {
		t.Fatal("disable did not persist", err)
	}
}
