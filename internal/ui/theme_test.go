package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestThemeFilesAndPartialOverrides(t *testing.T) {
	for _, file := range []string{"default.json", "gruvbox-dark.json", "hotdog.json", "c64.json"} {
		data, err := os.ReadFile(filepath.Join("themes", file))
		if err != nil {
			t.Fatal(err)
		}
		fields, err := themeObject(data)
		if err != nil {
			t.Fatal(err)
		}
		roles, err := themeObject(fields["colors"])
		if err != nil || len(roles) != reflect.TypeOf(Palette{}).NumField() {
			t.Fatal("example must define every role", file, err)
		}
		theme, err := LoadTheme("unused.toml", filepath.Join("themes", file))
		if err != nil || theme.Name == "" {
			t.Fatal(file, theme, err)
		}
		if file == "default.json" && theme.Palette != DefaultPalette() {
			t.Fatal("default JSON differs from startup")
		}
		if file == "gruvbox-dark.json" && (theme.Palette.Background != "#282828" || theme.Palette.Accent != "#8ec07c") {
			t.Fatal(theme)
		}
	}
	theme, err := ParseTheme([]byte(`{"version":1,"colors":{"text":"#abcdef","background":"#123456"}}`))
	want := DefaultPalette()
	want.Text, want.Background = "#abcdef", "#123456"
	if err != nil || theme.Palette != want {
		t.Fatal(theme, err)
	}
	theme, err = ParseTheme([]byte(`{"version":1,"colors":{"background":null}}`))
	if err != nil || theme.Palette != DefaultPalette() {
		t.Fatal(theme, err)
	}
	// All documented role names must load independently, including roles whose
	// defaults happen to be equal. Omitted neighbors keep their defaults.
	fields := reflect.TypeOf(Palette{})
	for i := 0; i < fields.NumField(); i++ {
		role := fields.Field(i).Tag.Get("json")
		theme, err := ParseTheme([]byte(`{"version":1,"colors":{"` + role + `":"#123456"}}`))
		if err != nil {
			t.Fatal(role, err)
		}
		want := DefaultPalette()
		reflect.ValueOf(&want).Elem().Field(i).SetString("#123456")
		if theme.Palette != want {
			t.Fatal("role override changed another role", role)
		}
	}
}

func TestThemeRejectsInvalidOrAmbiguousDocuments(t *testing.T) {
	for _, input := range []string{
		`null`, `[]`, `{}`, `{"version":2,"colors":{}}`, `{"version":"1","colors":{}}`,
		`{"version":1,"colors":null}`, `{"version":1,"colors":[]}`, `{"version":1,"name":null,"colors":{}}`,
		`{"version":1,"colors":{},"extra":true}`, `{"version":1,"Colors":{}}`,
		`{"version":1,"version":1,"colors":{}}`, `{"version":1,"colors":{"text":"#123456","text":"#abcdef"}}`,
		`{"version":1,"colors":{"Text":"#123456"}}`, `{"version":1,"colors":{"typo":"#123456"}}`,
		`{"version":1,"colors":{"text":null}}`, `{"version":1,"colors":{"text":123}}`,
		`{"version":1,"colors":{"text":"red"}}`, `{"version":1,"colors":{"background":""}}`,
		`{"version":1,"colors":{"text":"#fff"}}`, `{"version":1,"colors":{"text":"#12gg56"}}`,
		`{"version":1,"colors":{}} {}`, `{"version":1,"colors":{},}`, strings.Repeat(" ", maximumThemeBytes+1),
	} {
		if _, err := ParseTheme([]byte(input)); err == nil {
			t.Fatalf("accepted invalid theme %q", input[:min(len(input), 120)])
		}
	}
}

func TestThemePathsAndBoundedRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"name":"Custom","colors":{"info":"#123456"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{path, "custom.json"} {
		theme, err := LoadTheme(filepath.Join(dir, "config.toml"), file)
		if err != nil || theme.Name != "Custom" || theme.Palette.Info != "#123456" {
			t.Fatal(theme, err)
		}
	}
	if _, err := LoadTheme(filepath.Join(dir, "config.toml"), "missing.json"); err == nil {
		t.Fatal("missing theme silently fell back")
	}
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", maximumThemeBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTheme("unused.toml", path); err == nil {
		t.Fatal("oversize theme accepted")
	}
	if theme, err := LoadTheme("missing/config.toml", ""); err != nil || theme.Palette != DefaultPalette() {
		t.Fatal(theme, err)
	}
}

func TestThemeOverrideNamesAndWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.Mkdir("configuration", 0o700); err != nil {
		t.Fatal(err)
	}
	for path, name := range map[string]string{"custom theme.json": "Working directory", "configuration/custom theme.json": "Config directory"} {
		data := []byte(`{"version":1,"name":"` + name + `","colors":{"background":"#123456"}}`)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	theme, err := LoadThemeOverride("custom theme.json")
	if err != nil || theme.Name != "Working directory" {
		t.Fatal(theme, err)
	}
	theme, err = LoadTheme("configuration/config.toml", "custom theme.json")
	if err != nil || theme.Name != "Config directory" {
		t.Fatal(theme, err)
	}
	theme, err = LoadThemeOverride("default")
	if err != nil || theme.Palette != DefaultPalette() {
		t.Fatal(theme, err)
	}
	theme, err = LoadThemeOverride("gruvbox-dark")
	if err != nil || theme.Name != "Gruvbox Dark" || theme.Palette.Background != "#282828" {
		t.Fatal(theme, err)
	}
	theme, err = LoadThemeOverride("hotdog")
	if err != nil || theme.Name != "Hotdog" || theme.Palette.Background != "#FFFDE7" {
		t.Fatal(theme, err)
	}
	theme, err = LoadThemeOverride("c64")
	if err != nil || theme.Name != "C64" || theme.Palette.Background != "#352879" || theme.Palette.SelectedBackground != "#6C5EB5" {
		t.Fatal(theme, err)
	}
	for _, name := range []string{"", "missing.json", "gruvbox-dakr"} {
		if _, err := LoadThemeOverride(name); err == nil {
			t.Fatal("invalid override silently accepted", name)
		}
	}
}

func TestThemeCanvasCoversEverySurfaceAndResize(t *testing.T) {
	profile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	for _, name := range []string{"gruvbox-dark", "hotdog", "c64"} {
		theme, err := LoadThemeOverride(name)
		if err != nil {
			t.Fatal(err)
		}
		p := theme.Palette
		for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
			lipgloss.SetColorProfile(profile)
			for _, route := range []string{"dashboard", "?", "enter", "m", ",", "a", "N", "D", "ctrl+e"} {
				m := demoModel().WithPalette(p)
				if route != "dashboard" {
					m = key(m, route)
				}
				for _, size := range [][2]int{{60, 18}, {59, 17}, {120, 32}} {
					m = resizeModel(m, size[0], size[1])
					view := m.View()
					if size[0] == 59 {
						assertPaint(t, view, "Resize to at least", p.Text, p.Background, false)
					}
					lines := strings.Split(view, "\n")
					if len(lines) != size[1] {
						t.Fatal(route, size, "height", len(lines))
					}
					for _, line := range lines {
						if ansi.StringWidth(line) != size[0] {
							t.Fatal(route, size, "width", ansi.StringWidth(line))
						}
					}
					allowed := map[string]bool{}
					for _, bg := range []lipgloss.Color{p.Background, p.SelectedBackground, p.FooterBackground} {
						allowed[expectedPaint(p.Text, bg, false).background] = true
					}
					for _, cell := range paintedCells(view) {
						if !allowed[cell.background] || cell.foreground == "default" && profile != termenv.Ascii {
							t.Fatal(route, size, profile, "unpainted cell", cell)
						}
					}
					if profile == termenv.Ascii && strings.Contains(view, "\x1b") {
						t.Fatal("ANSI escaped into no-color output")
					}
				}
			}
		}
	}
}

func TestSurfaceRestoresDefaultsWithoutReplacingExplicitColors(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	p := DefaultPalette()
	// Zero and 49 inside RGB components are data, never reset attributes.
	text := "plain\x1b[1;38;2;0;49;255mcolored\x1b[39mdefault\x1b[48;2;49;0;0mselected\x1b[49mafter\x1b[0mreset"
	got := paintSurface(text, p.Text, p.SelectedBackground)
	assertPaint(t, got, "plain", p.Text, p.SelectedBackground, false)
	cells := paintedCells(got)
	for _, cell := range cells[5:12] {
		if cell.foreground != "0000/3131/ffff/ffff" || cell.background != expectedPaint(p.Text, p.SelectedBackground, false).background || !cell.bold {
			t.Fatal("RGB components treated as resets", cell)
		}
	}
	assertPaint(t, got, "default", p.Text, p.SelectedBackground, true)
	for _, cell := range cells[19:27] {
		if cell.background != "3131/0000/0000/ffff" || cell.foreground != expectedPaint(p.Text, "", false).foreground || !cell.bold {
			t.Fatal("explicit background replaced", cell)
		}
	}
	assertPaint(t, got, "after", p.Text, p.SelectedBackground, true)
	assertPaint(t, got, "reset", p.Text, p.SelectedBackground, false)
}
