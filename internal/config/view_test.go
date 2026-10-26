package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestViewFallbackAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	for _, tc := range []struct {
		input string
		want  ViewPreferences
	}{
		{"", ViewPreferences{FilterAll, SortName}},
		{"[view]\nfilter='printing'\nsort='finish'", ViewPreferences{FilterPrinting, SortFinish}},
		{"[view]\nfilter='future-filter'\nsort='state'", ViewPreferences{FilterAll, SortState}},
		{"[view]\nfilter='attention'\nsort='future-sort'", ViewPreferences{FilterAttention, SortName}},
		{"[view]\nfilter=42\nsort=false", ViewPreferences{FilterAll, SortName}},
	} {
		if err := os.WriteFile(path, []byte(tc.input), 0o600); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load(path)
		if err != nil || loaded.View != tc.want {
			t.Fatalf("%s: %+v %v", tc.input, loaded, err)
		}
	}
	before, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, filter := range []ViewChoice{FilterAll, FilterAttention, FilterPrinting, FilterUnavailable} {
		for _, sort := range []ViewChoice{SortName, SortAttention, SortState, SortFinish} {
			after := before.Clone()
			after.View = ViewPreferences{filter, sort}
			if err := SaveChanges(path, before, after); err != nil {
				t.Fatal(err)
			}
			loaded, err := Load(path)
			if err != nil || loaded.View != after.View {
				t.Fatal(loaded, err)
			}
			before = after
		}
	}
}

func TestViewMergePreservesIndependentExternalEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("region='china'\n[view]\nfilter='attention'\nsort='finish'\n[alerts]\nnotification_sound=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	after := before.Clone()
	after.View.Filter = FilterPrinting
	if err := SaveChanges(path, before, after); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.View != (ViewPreferences{FilterPrinting, SortFinish}) || loaded.Region != RegionChina || !loaded.Alerts.NotificationSound {
		t.Fatal(loaded, err)
	}
	before = loaded.Clone()
	after = before.Clone()
	after.Alerts.Filament = false
	if err := SaveChanges(path, before, after); err != nil {
		t.Fatal(err)
	}
	loaded, err = Load(path)
	if err != nil || loaded.View != before.View {
		t.Fatal("Config save lost view", loaded, err)
	}
}
