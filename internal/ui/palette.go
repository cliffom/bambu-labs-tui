package ui

import "github.com/charmbracelet/lipgloss"

type (
	// Palette defines application colors by role. Unstyled body prose inherits
	// terminal colors; reported filament colors are telemetry, outside this palette.
	// Equal default colors are separate roles so future themes can vary them.
	Palette struct {
		Text, SecondaryText, MutedText                       lipgloss.Color
		Info, Accent, Success, Warning, Error, Focus, Border lipgloss.Color
		SelectedText, SelectedBackground, Hotkey             lipgloss.Color
		FooterBackground, FooterText, FooterMuted            lipgloss.Color
		FooterInfo, FooterSuccess, FooterWarning             lipgloss.Color
		FooterError, FooterHotkey, FooterSeparator           lipgloss.Color
	}

	styles struct {
		text, secondary, muted, info, accent, success, warning, err lipgloss.Style
		focus, selection, border, hotkey                            lipgloss.Style
		footer, footerHotkey, footerSeparator                       lipgloss.Style
		footerInfo, footerSuccess, footerWarning, footerError       lipgloss.Style
		footerMuted                                                 lipgloss.Style
	}
)

// DefaultPalette returns an independent value, never shared mutable theme state.
func DefaultPalette() Palette {
	return Palette{
		Text: "#E4E9F0", SecondaryText: "#929BA8", MutedText: "#929BA8",
		Info: "#E4E9F0", Accent: "#A3E4BB", Success: "#A3E4BB", Warning: "#F4BA72", Error: "#FF8087",
		Focus: "#A3E4BB", Border: "#A3E4BB",
		SelectedText: "#D9F5E4", SelectedBackground: "#263B36", Hotkey: "#E4E9F0",
		FooterBackground: "#3A4652", FooterText: "#B8C2CE", FooterMuted: "#929BA8",
		FooterInfo: "#E4E9F0", FooterSuccess: "#A3E4BB", FooterWarning: "#F4BA72",
		FooterError: "#FF9AA1", FooterHotkey: "#E4E9F0", FooterSeparator: "#B8C2CE",
	}
}

func newStyles(p Palette) *styles {
	text := func(color lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(color) }
	foot := func(color lipgloss.Color) lipgloss.Style { return text(color).Background(p.FooterBackground) }
	return &styles{
		text: text(p.Text), secondary: text(p.SecondaryText), muted: text(p.MutedText),
		info: text(p.Info).Bold(true), accent: text(p.Accent), success: text(p.Success), warning: text(p.Warning), err: text(p.Error),
		focus: text(p.Focus), selection: text(p.SelectedText).Background(p.SelectedBackground),
		border: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(p.Border).Padding(0, 1),
		hotkey: text(p.Hotkey).Bold(true), footer: foot(p.FooterText),
		footerHotkey: foot(p.FooterHotkey).Bold(true), footerSeparator: foot(p.FooterSeparator),
		footerInfo: foot(p.FooterInfo).Bold(true), footerSuccess: foot(p.FooterSuccess).Bold(true),
		footerWarning: foot(p.FooterWarning).Bold(true), footerError: foot(p.FooterError).Bold(true),
		footerMuted: foot(p.FooterMuted),
	}
}

// WithPalette changes presentation only. Rebuild cached viewport content without
// running commands, changing feedback lifetimes, or opening automatic dialogs.
func (m Model) WithPalette(p Palette) Model {
	m.styles = newStyles(p)
	// Dialogs own cached styled viewports through pointers. Do not recolor an
	// earlier model value's cache while constructing a differently styled value.
	m.attention = cloneDialog(m.attention)
	m.filament = cloneDialog(m.filament)
	m.decisions = cloneDialog(m.decisions)
	m.alertDialog = cloneDialog(m.alertDialog)
	m.notificationActivity = cloneDialog(m.notificationActivity)
	m.exportDialog = cloneDialog(m.exportDialog)
	m.syncPanels()
	for _, kind := range []dialogKind{m.dialog.resume, m.dialog.returnTo, m.dialog.active} {
		if sync := routeFor(kind).sync; sync != nil {
			sync(&m)
		}
	}
	return m
}

func cloneDialog[T any](source *T) *T {
	if source == nil {
		return nil
	}
	copy := *source
	return &copy
}
