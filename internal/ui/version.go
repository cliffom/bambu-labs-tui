package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambustand/internal/buildinfo"
)

// versionLabel uses the same build metadata as --version, bounded for terminal layout.
func versionLabel() string {
	version := safe(buildinfo.Version)
	if version == "" {
		version = "unknown"
	}
	return "v" + ansi.Truncate(version, versionLabelMaxWidth, "…")
}

// versionedTitle keeps the running version visible in dialog headings.
func (m Model) versionedTitle(title string, width int) string {
	suffix := m.styles.secondary.Render(" · " + versionLabel())
	return ansi.Truncate(title, max(0, width-ansi.StringWidth(suffix)), "…") + suffix
}

// dashboardHeader gives the version and connection state priority over the clock.
func (m Model) dashboardHeader() string {
	mode := "CLOUD"
	if m.Demo {
		mode = "DEMO"
	}
	connection := m.styles.warning.Render("CONNECTING")
	if m.Farm.Connected {
		connection = m.styles.accent.Render("CONNECTED")
	}
	if m.ended && !m.Demo {
		connection = m.styles.err.Render("SIGN-IN REQUIRED")
	}
	prefix := m.styles.accent.Bold(true).Render("BAMBU STAND ")
	suffix := m.styles.secondary.Render(" / "+mode+" ") + connection + m.styles.secondary.Render(" / FARM")
	version := ansi.Truncate(versionLabel(), max(1, m.width-ansi.StringWidth(prefix+suffix)), "…")
	header := prefix + m.styles.secondary.Render(version) + suffix
	clock := m.now.Local().Format("Mon 02 Jan · 15:04:05")
	available := m.width - ansi.StringWidth(header) - 1
	if ansi.StringWidth(clock) > available {
		clock = m.now.Local().Format(time.TimeOnly)
	}
	if ansi.StringWidth(clock) <= available {
		header += strings.Repeat(" ", max(1, m.width-ansi.StringWidth(header)-ansi.StringWidth(clock))) + m.styles.secondary.Render(clock)
	}
	return header
}
