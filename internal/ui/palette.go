package ui

import "github.com/charmbracelet/lipgloss"

type (
	// Palette defines application colors by role. An empty Background inherits
	// the terminal background; reported filament colors remain telemetry.
	// Equal default colors are separate roles so future themes can vary them.
	Palette struct {
		Text               lipgloss.Color `json:"text"`
		Background         lipgloss.Color `json:"background"`
		SecondaryText      lipgloss.Color `json:"secondary_text"`
		MutedText          lipgloss.Color `json:"muted_text"`
		Info               lipgloss.Color `json:"info"`
		Accent             lipgloss.Color `json:"accent"`
		Success            lipgloss.Color `json:"success"`
		Warning            lipgloss.Color `json:"warning"`
		Error              lipgloss.Color `json:"error"`
		Focus              lipgloss.Color `json:"focus"`
		Border             lipgloss.Color `json:"border"`
		SelectedText       lipgloss.Color `json:"selected_text"`
		SelectedBackground lipgloss.Color `json:"selected_background"`
		Hotkey             lipgloss.Color `json:"hotkey"`
		FooterBackground   lipgloss.Color `json:"footer_background"`
		FooterText         lipgloss.Color `json:"footer_text"`
		FooterMuted        lipgloss.Color `json:"footer_muted"`
		FooterInfo         lipgloss.Color `json:"footer_info"`
		FooterSuccess      lipgloss.Color `json:"footer_success"`
		FooterWarning      lipgloss.Color `json:"footer_warning"`
		FooterError        lipgloss.Color `json:"footer_error"`
		FooterHotkey       lipgloss.Color `json:"footer_hotkey"`
		FooterSeparator    lipgloss.Color `json:"footer_separator"`
	}

	styles struct {
		text, secondary, muted, info, accent, success, warning, err lipgloss.Style
		focus, selection, border, hotkey                            lipgloss.Style
		footer, footerHotkey, footerSeparator                       lipgloss.Style
		footerInfo, footerSuccess, footerWarning, footerError       lipgloss.Style
		footerMuted                                                 lipgloss.Style
		palette                                                     Palette
	}
)

// DefaultPalette returns an independent value, never shared mutable theme state.
func DefaultPalette() Palette {
	theme, err := ParseTheme(defaultTheme)
	if err != nil {
		panic("invalid embedded default theme: " + err.Error())
	}
	return theme.Palette
}

func newStyles(p Palette) *styles {
	text := func(color lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(color) }
	foot := func(color lipgloss.Color) lipgloss.Style { return text(color).Background(p.FooterBackground) }
	return &styles{
		palette: p,
		text:    text(p.Text), secondary: text(p.SecondaryText), muted: text(p.MutedText),
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
