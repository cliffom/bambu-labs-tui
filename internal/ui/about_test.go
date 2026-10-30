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
	"github.com/cliffom/bambustand/internal/buildinfo"
	"github.com/muesli/termenv"
)

func TestAboutMetadata(t *testing.T) {
	m := key(resizeModel(demoModel(), 60, 18), "A")
	content := ansi.Strip(m.about.View())
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macOS"
	}
	previous := -1
	for _, want := range []string{"BambuStand " + buildinfo.Version, platform + " / " + runtime.GOARCH, "https://github.com/cliffom/bambustand", "A Suite Potato Spud", "Lovingly crafted by Michael Clifford", "Theme: default", "Theme Palette:"} {
		at := strings.Index(content, want)
		if at <= previous {
			t.Fatalf("missing or out-of-order %q in %q", want, content)
		}
		previous = at
	}
	for _, label := range []string{"Application:", "Version:", "Platform:", "Project:", "Login email:"} {
		if strings.Contains(content, label) {
			t.Fatal("removed field still shown", label)
		}
	}
}

func TestAboutFieldColors(t *testing.T) {
	previous := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(previous)
	for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
		lipgloss.SetColorProfile(profile)
		m := key(resizeModel(demoModel(), 60, 18), "A")
		p := DefaultPalette()
		p.Text, p.SecondaryText, p.Background = "#FFFFFF", "#808080", "#101010"
		m = m.WithPalette(p)
		view := m.View()
		assertPaint(t, view, "Theme:", p.SecondaryText, p.Background, false)
		assertPaint(t, view, "Theme Palette:", p.SecondaryText, p.Background, false)
		assertPaint(t, view, "default", p.Text, p.Background, false)
	}
}

func TestPaletteColorsDeduplicatesAndSortsByHue(t *testing.T) {
	p := Palette{Text: "#ff0000", SecondaryText: "#FF0000", Accent: "#00FF00", Info: "#0000FF", Warning: "#FFFF00", Error: "#FF00FF", Focus: "#00FFFF", MutedText: "#808080", Border: "#000000", Success: "#FFFFFF"}
	want := []lipgloss.Color{"#000000", "#808080", "#FFFFFF", "#FF0000", "#FFFF00", "#00FF00", "#00FFFF", "#0000FF", "#FF00FF"}
	if got := paletteColors(p); !slices.Equal(got, want) {
		t.Fatal("unexpected unique hue order", got)
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
			unique := make(map[string]bool)
			palette := reflect.ValueOf(theme.Palette)
			for i := 0; i < palette.NumField(); i++ {
				color := strings.ToUpper(palette.Field(i).String())
				if color != "" {
					unique[color] = true
				}
				role := palette.Type().Field(i).Tag.Get("json")
				if strings.Contains(grid, role) {
					t.Fatal("role label in unlabeled grid", role)
				}
			}
			if strings.Count(grid, "█") != len(unique)*aboutSwatchWidth || strings.Contains(grid, "#") || strings.Contains(grid, "inherited") {
				t.Fatal("grid does not contain only unique swatches", grid)
			}
			for _, line := range strings.Split(grid, "\n") {
				if ansi.StringWidth(line) > size[0]-2 {
					t.Fatal("grid overflow", line)
				}
			}

		}
	}
}

func TestAboutFocusScrollAndReturn(t *testing.T) {
	for _, closeKey := range []string{"esc", "enter", "A"} {
		m := resizeModel(demoModel(), 60, 18)
		m.activeTheme = strings.Repeat("long/custom/path/", 60)
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
		if offset == 0 || !strings.Contains(ansi.Strip(m.View()), "█") {
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
		original := paletteColors(m.styles.palette)
		m = m.WithPalette(p)
		if !slices.Contains(paletteColors(m.styles.palette), lipgloss.Color("#ABCDEF")) {
			t.Fatal("cached palette did not refresh")
		}
		if m.dialog.active != dialogAbout {
			t.Fatal("recolor changed owner")
		}
		if slices.Equal(original, paletteColors(m.styles.palette)) {
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
	if !slices.Contains(paletteColors(m.styles.palette), lipgloss.Color("#123456")) {
		t.Fatal("configured palette lost")
	}
}
