package ui

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/buildinfo"
)

// WithLoginEmail supplies only the account information displayed by About.
// Tokens and transport identities never enter the UI model.
func (m Model) WithLoginEmail(email string) Model {
	m.loginEmail = email
	return m
}

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
	email := safe(m.loginEmail)
	switch {
	case m.Demo:
		email = "Demo mode (not signed in)"
	case m.ended:
		email = "Not signed in"
	case strings.TrimSpace(email) == "":
		email = "Unavailable (login email not recorded)"
	}
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
		"Application: Bambu TUI",
		"Version: " + version,
		"Platform: " + platform + " / " + runtime.GOARCH,
		"Login email: " + email,
		"Theme: " + theme,
		"Project: https://github.com/cliffom/bambu-tui",
		"Loving Crafted by Michael Clifford",
		"A Suite Potato Spud",
		"",
	}
	content := ansi.Wrap(strings.Join(lines, "\n"), m.about.Width, "")
	m.about.SetContent(content + "\n" + m.aboutPalette())
}

// Every effective role appears once, even when several roles share RGB values.
// Two-line cells keep role names and literal colors legible without color.
func (m Model) aboutPalette() string {
	palette := reflect.ValueOf(m.styles.palette)
	columns := max(1, m.about.Width/aboutPaletteCellWidth)
	rows := (palette.NumField() + columns - 1) / columns
	lines := []string{fmt.Sprintf("Theme colors: %d×%d grid (columns × rows)", columns, rows)}
	for row := 0; row < rows; row++ {
		var labels, swatches strings.Builder
		for col := 0; col < columns; col++ {
			i := row*columns + col
			if i >= palette.NumField() {
				break
			}
			role := palette.Type().Field(i).Tag.Get("json")
			color := lipgloss.Color(palette.Field(i).String())
			value := string(color)
			swatch := lipgloss.NewStyle().Background(color).Render("  ")
			if color == "" {
				value, swatch = "terminal (inherited)", "--"
			}
			labels.WriteString(cell(role, aboutPaletteCellWidth))
			rendered := swatch + " " + value
			swatches.WriteString(rendered + strings.Repeat(" ", max(0, aboutPaletteCellWidth-ansi.StringWidth(rendered))))
		}
		lines = append(lines, labels.String(), swatches.String())
	}
	return strings.Join(lines, "\n")
}

func (m Model) aboutView() string {
	return m.fullView([]string{m.styles.accent.Bold(true).Render("ABOUT"), m.styles.secondary.Render("↑/↓ or PgUp/PgDn scroll · Home/End · Enter/Esc back"), "", m.about.View()}, "↑/↓ scroll  Enter/Esc back")
}
