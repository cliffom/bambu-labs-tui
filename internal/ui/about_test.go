package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/buildinfo"
	"github.com/muesli/termenv"
)

func TestAboutAccountAndMetadata(t *testing.T) {
	for _, tc := range []struct {
		email       string
		demo, ended bool
		want        string
	}{
		{"operator@example.test", false, false, "operator@example.test"},
		{"", false, false, "Unavailable (login email not recorded)"},
		{"ignored@example.test", true, false, "Demo mode (not signed in)"},
		{"ignored@example.test", false, true, "Not signed in"},
		{strings.Repeat("long", 35) + "@example.test", false, false, strings.Repeat("long", 35) + "@example.test"},
	} {
		m := demoModel().WithLoginEmail(tc.email)
		m.Demo, m.ended = tc.demo, tc.ended
		m = key(resizeModel(m, 60, 18), "A")
		// Rejoin wrapped lines so arbitrarily long addresses remain inspectable.
		m.about.GotoTop()
		m.about.Height = m.about.TotalLineCount()
		content := strings.ReplaceAll(ansi.Strip(m.about.View()), "\n", "")
		content = strings.ReplaceAll(content, " ", "")
		for _, want := range []string{tc.want, "Bambu TUI", buildinfo.Version, runtime.GOARCH, "https://github.com/cliffom/bambu-tui", "Loving Crafted by Michael Clifford", "A Suite Potato Spud"} {
			if !strings.Contains(content, strings.ReplaceAll(want, " ", "")) {
				t.Fatalf("missing %q in %q", want, content)
			}
		}
		if tc.demo || tc.ended {
			if strings.Contains(content, "ignored@example.test") {
				t.Fatal("inactive email exposed")
			}
		}
	}
}

func TestAboutEffectiveThemeAndGrid(t *testing.T) {
	names, err := BundledThemeNames()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "custom.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"name":"Unrelated label","colors":{"accent":"#123456"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	names = append(names, path)
	for _, choice := range names {
		theme, err := LoadThemeOverride(choice)
		if err != nil {
			t.Fatal(err)
		}
		for _, size := range [][2]int{{60, 18}, {120, 32}} {
			m := demoModel().WithTheme(theme)
			m.configThemeFile = "saved-but-overridden.json"
			m = key(resizeModel(m, size[0], size[1]), "A")
			m.about.Height = m.about.TotalLineCount()
			text := ansi.Strip(m.about.View())
			if !strings.Contains(strings.Join(strings.Fields(text), ""), "Theme:"+strings.ReplaceAll(choice, " ", "")) || strings.Contains(text, m.configThemeFile) {
				t.Fatal("effective theme lost", text)
			}
			grid := ansi.Strip(m.aboutPalette())
			palette := reflect.ValueOf(theme.Palette)
			for i := 0; i < palette.NumField(); i++ {
				role := palette.Type().Field(i).Tag.Get("json")
				if !slices.Contains(strings.Fields(grid), role) {
					t.Fatal("missing/duplicated role", role, grid)
				}
				color := palette.Field(i).String()
				if color == "" {
					color = "terminal (inherited)"
				}
				if !strings.Contains(grid, color) {
					t.Fatal("missing effective color", color, grid)
				}
			}
			for _, line := range strings.Split(grid, "\n") {
				if ansi.StringWidth(line) > size[0]-2 {
					t.Fatal("grid overflow", line)
				}
			}
			if choice == path && !strings.Contains(grid, "#123456") {
				t.Fatal("custom override lost")
			}
		}
	}
}

func TestAboutFocusScrollAndReturn(t *testing.T) {
	for _, closeKey := range []string{"esc", "enter", "A"} {
		m := resizeModel(demoModel(), 60, 18)
		m = key(key(m, "j"), "tab")
		selection, focus := m.selection, m.focus
		m = key(m, "A")
		for _, blocked := range []string{"q", "ctrl+c", "tab", "F", "a"} {
			m = key(m, blocked)
		}
		if m.dialog.active != dialogAbout || m.selection != selection || m.focus != focus {
			t.Fatal("About leaked input")
		}
		m = key(m, "end")
		offset := m.about.YOffset
		if offset == 0 || !strings.Contains(ansi.Strip(m.View()), "footer_separator") {
			t.Fatal("palette cannot scroll")
		}
		m = key(m, "ctrl+e")
		m = key(m, "esc")
		if m.dialog.active != dialogAbout || m.about.YOffset != offset {
			t.Fatal("status lost About position")
		}
		m = resizeModel(m, 59, 17)
		for _, blocked := range []string{"A", "enter", "home", "j"} {
			m = key(m, blocked)
		}
		m = resizeModel(m, 60, 18)
		if m.about.YOffset != offset {
			t.Fatal("fallback lost scroll")
		}
		m = key(m, closeKey)
		if m.hasOpenDialog() || m.selection != selection || m.focus != focus {
			t.Fatal("close lost dashboard position")
		}
	}
}

func TestAboutPaletteProfilesAndRecoloring(t *testing.T) {
	previous := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(previous)
	for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
		lipgloss.SetColorProfile(profile)
		m := key(resizeModel(demoModel(), 60, 18), "A")
		p := DefaultPalette()
		p.Text, p.Background, p.Accent = "#123456", "#101010", "#ABCDEF"
		original := m.aboutPalette()
		m = m.WithPalette(p)
		if !strings.Contains(ansi.Strip(m.aboutPalette()), "#ABCDEF") {
			t.Fatal("cached palette did not refresh")
		}
		if m.dialog.active != dialogAbout {
			t.Fatal("recolor changed owner")
		}
		if original == m.aboutPalette() {
			t.Fatal("palette unchanged")
		}
		for _, size := range [][2]int{{60, 18}, {120, 32}, {59, 17}, {60, 18}} {
			m = resizeModel(m, size[0], size[1])
			lines := strings.Split(m.View(), "\n")
			if len(lines) != size[1] {
				t.Fatal("height overflow", size)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) != size[0] {
					t.Fatal("width overflow", size, line)
				}
			}
		}
		if profile == termenv.Ascii && strings.Contains(m.View(), "\x1b") {
			t.Fatal("color escaped into no-color output")
		}
		if profile != termenv.Ascii {
			found := false
			for _, c := range paintedCells(m.aboutPalette()) {
				if c.background == expectedPaint(p.Text, p.Accent, false).background {
					found = true
				}
			}
			if !found {
				t.Fatal("accent swatch missing its background")
			}
		}
	}
}

func TestAboutConfiguredCustomSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "personal theme.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"colors":{"accent":"#123456"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	theme, err := LoadTheme(filepath.Join(dir, "config.toml"), "personal theme.json")
	if err != nil {
		t.Fatal(err)
	}
	if theme.Source != path {
		t.Fatal("configuration-relative source lost", theme.Source)
	}
	m := key(demoModel().WithTheme(theme), "A")
	if !strings.Contains(ansi.Strip(m.aboutPalette()), "#123456") {
		t.Fatal("configured palette lost")
	}
}
