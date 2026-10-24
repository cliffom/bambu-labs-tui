package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestModalQuitKeys requires leaving dialogs before global quit keys take effect.
func TestModalQuitKeys(t *testing.T) {
	for _, dialog := range []string{"attention", "filament", "decisions", "reset", "inspection", "confirmation", "help"} {
		t.Run(dialog, func(t *testing.T) {
			m := demoModel()
			switch dialog {
			case "attention":
				m = attentionTick(attentionReport(t, m, "H", runoutReport))
			case "inspection":
				m = noticeModel(t)
				m = key(m, "enter")
			case "decisions", "reset":
				m = key(m, "n")
				m.decisions.confirmReset = dialog == "reset"
			case "filament":
				m = key(m, "m")
			case "confirmation":
				m.confirmClear = true
			case "help":
				m.help = true
			}
			for _, input := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyCtrlC}} {
				next, cmd := m.Update(input)
				m = next.(Model)
				if cmd != nil || (m.attention == nil && m.filament == nil && m.decisions == nil && m.inspected == nil && !m.confirmClear && !m.help) {
					t.Fatal("quit key escaped or closed a modal")
				}
			}
			m = key(m, "esc")
			if dialog == "reset" {
				m = key(m, "esc")
			}
			for _, input := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyCtrlC}} {
				_, cmd := m.Update(input)
				if cmd == nil {
					t.Fatal("dashboard quit stopped working")
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatal("dashboard quit did not return a quit message")
				}
			}
		})
	}
}
