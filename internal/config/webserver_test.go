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

func TestWebPortDefaultsAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, body := range []string{"", "[web_server]\nenabled=true\n"} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil || got.WebServer.Port != DefaultWebPort {
			t.Fatal("legacy config did not receive default port", got, err)
		}
	}
	for _, port := range []string{"0", "-1", "65536", "1.5", `"8080"`} {
		if err := os.WriteFile(path, []byte("[web_server]\nport="+port+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("accepted invalid web port", port)
		}
	}
	for _, port := range []string{"1", "65535", "9090"} {
		if err := os.WriteFile(path, []byte("[web_server]\nport="+port+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err != nil {
			t.Fatal("rejected valid port", port, err)
		}
	}
}

func TestWebPortAndEnabledMergeIndependently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[web_server]\nenabled=true\nport=8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.WebServer.Port = 9090
	if err := SaveChanges(path, before, after); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !got.WebServer.Enabled || got.WebServer.Port != 9090 {
		t.Fatal("port edit lost external enabled setting", got, err)
	}
	before = got.Clone()
	if err := os.WriteFile(path, []byte("[web_server]\nenabled=true\nport=10000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	after = before.Clone()
	after.WebServer.Enabled = false
	if err := SaveChanges(path, before, after); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path)
	if err != nil || got.WebServer.Enabled || got.WebServer.Port != 10000 {
		t.Fatal("enabled edit lost external port setting", got, err)
	}
}
