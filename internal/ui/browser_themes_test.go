package ui

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestBrowserThemesReuseAllTerminalRolesWithoutPrivateMetadata(t *testing.T) {
	theme, err := ParseTheme([]byte(`{"version":1,"name":"private theme metadata","colors":{"background":"#123456","accent":"#ABCDEF"}}`))
	if err != nil {
		t.Fatal(err)
	}
	theme.Source = "/private/account/theme.json"
	server := &fakeWebServer{}
	m := demoModel().WithTheme(theme).WithWebServer(server)
	if server.themes.Themes[0].Colors["background"] != "#123456" || server.themes.Themes[0].Colors["accent"] != "#ABCDEF" {
		t.Fatal(server.themes)
	}
	names, _ := BundledThemeNames()
	if len(server.themes.Themes) != len(names)+1 {
		t.Fatal("inventory diverged")
	}
	for i, name := range names {
		theme, err := LoadThemeOverride(name)
		if err != nil {
			t.Fatal(err)
		}
		want := reflect.ValueOf(theme.Palette)
		got := server.themes.Themes[i+1]
		if got.ID != name {
			t.Fatal(got.ID, name)
		}
		for field := 0; field < want.NumField(); field++ {
			role := want.Type().Field(field).Tag.Get("json")
			if got.Colors[role] != want.Field(field).String() {
				t.Fatal(name, role)
			}
		}
	}
	encoded, err := json.Marshal(server.themes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private") {
		t.Fatal("private metadata escaped", string(encoded))
	}
	p := m.styles.palette
	p.Accent = "#112233"
	m = m.WithPalette(p)
	if server.themes.Themes[0].Colors["accent"] != "#112233" || m.styles.palette.Accent != "#112233" {
		t.Fatal("host recoloring not published")
	}
	// Attaching before WithTheme must produce the same effective host selection.
	other := &fakeWebServer{}
	_ = demoModel().WithWebServer(other).WithTheme(theme)
	if other.themes.Themes[0].Colors["background"] != "#123456" {
		t.Fatal(other.themes)
	}
}
