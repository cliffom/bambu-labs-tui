package ui

import (
	"cmp"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/buildinfo"
)

// WithTheme retains the effective launch choice rather than the saved preference.
func (m Model) WithTheme(theme Theme) Model {
	m.activeTheme = theme.Source
	if m.activeTheme == "" {
		m.activeTheme = theme.Name
	}
	return m.WithPalette(theme.Palette)
}

func (m *Model) openAbout() {
	m.about = viewport.New(1, 1)
	m.openDialog(dialogAbout)
	m.syncAbout()
}

func (m *Model) aboutKey(key string, msg tea.Msg) tea.Cmd {
	switch key {
	case keyEscape, keyEnter, keyAbout:
		m.closeDialog()
	case keyQuit, keyInterrupt:
		return nil
	case keyHome, keyFirst:
		m.about.GotoTop()
	case keyEnd, keyLast:
		m.about.GotoBottom()
	default:
		var cmd tea.Cmd
		m.about, cmd = m.about.Update(msg)
		return cmd
	}
	return nil
}

func (m *Model) syncAbout() {
	m.about.Width, m.about.Height = max(1, m.width-2), max(1, m.height-4)
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macOS"
	}
	theme := safe(m.activeTheme)
	if theme == "" {
		theme = "default"
	}
	version := safe(buildinfo.Version)
	if version == "" {
		version = "unknown"
	}
	lines := []string{
		"Bambu TUI " + version,
		platform + " / " + runtime.GOARCH,
		"https://github.com/cliffom/bambu-tui",
		"A Suite Potato Spud",
		"Lovingly crafted by Michael Clifford",
		"",
		"Theme: " + theme,
		"Theme Pallet:",
	}
	content := ansi.Wrap(strings.Join(lines, "\n"), m.about.Width, "")
	m.about.SetContent(content + "\n" + m.aboutPalette())
}

// paletteColors omits the inherited terminal color, whose RGB is unknown.
// Normalize case before deduplicating roles that share the same effective RGB.
func paletteColors(p Palette) []lipgloss.Color {
	palette := reflect.ValueOf(p)
	seen := make(map[lipgloss.Color]bool)
	var colors []lipgloss.Color
	for i := 0; i < palette.NumField(); i++ {
		color := lipgloss.Color(strings.ToUpper(palette.Field(i).String()))
		if color != "" && !seen[color] {
			seen[color] = true
			colors = append(colors, color)
		}
	}
	slices.SortFunc(colors, func(a, b lipgloss.Color) int {
		ah, as, av := paletteHSV(a)
		bh, bs, bv := paletteHSV(b)
		if order := cmp.Compare(ah, bh); order != 0 {
			return order
		}
		if order := cmp.Compare(as, bs); order != 0 {
			return order
		}
		if order := cmp.Compare(av, bv); order != 0 {
			return order
		}
		return cmp.Compare(a, b)
	})
	return colors
}

// Achromatic colors precede the hue wheel, ordered from black to white.
// Theme loading validates RGB syntax before palettes reach the UI.
func paletteHSV(color lipgloss.Color) (hue, saturation, value float64) {
	rgb, _ := strconv.ParseUint(strings.TrimPrefix(string(color), "#"), 16, 24)
	r, g, b := float64(rgb>>16), float64((rgb>>8)&255), float64(rgb&255)
	high, low := max(r, g, b), min(r, g, b)
	delta := high - low
	if delta == 0 {
		return -1, 0, high
	}
	switch high {
	case r:
		hue = (g - b) / delta
	case g:
		hue = (b-r)/delta + 2
	default:
		hue = (r-g)/delta + 4
	}
	if hue < 0 {
		hue += 6
	}
	return hue, delta / high, high
}

func (m Model) aboutPalette() string {
	colors := paletteColors(m.styles.palette)
	columns := max(1, m.about.Width/aboutPaletteCellWidth)
	var lines []string
	for start := 0; start < len(colors); start += columns {
		var row []string
		for _, color := range colors[start:min(start+columns, len(colors))] {
			// Solid glyphs preserve the grid shape when ANSI colors are disabled.
			row = append(row, lipgloss.NewStyle().Foreground(color).Background(color).Render(strings.Repeat("█", aboutSwatchWidth)))
		}
		lines = append(lines, strings.Join(row, strings.Repeat(" ", aboutPaletteCellWidth-aboutSwatchWidth)))
	}
	return strings.Join(lines, "\n\n")
}

func (m Model) aboutView() string {
	return m.fullView([]string{m.styles.accent.Bold(true).Render("ABOUT"), m.styles.secondary.Render("↑/↓ or PgUp/PgDn scroll · Home/End · Enter/Esc back"), "", m.about.View()}, "↑/↓ scroll  Enter/Esc back")
}
