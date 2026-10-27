package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/cliffom/bambu-tui/internal/notifications"
	"github.com/muesli/termenv"
)

type paintedCell struct {
	foreground, background string
	bold                   bool
}

// Inspect the emitted terminal state at every printed cell, including ANSI
// resets. This catches incorrect roles and background gaps after nested styles.
func paintedCells(text string) []paintedCell {
	var cells []paintedCell
	var state byte
	var foreground, background color.Color
	bold := false
	p := ansi.NewParser()
	key := func(c color.Color) string {
		if c == nil {
			return "default"
		}
		r, g, b, a := c.RGBA()
		return fmt.Sprintf("%04x/%04x/%04x/%04x", r, g, b, a)
	}
	for len(text) > 0 {
		seq, width, n, next := ansi.DecodeSequence(text, state, p)
		text, state = text[n:], next
		if width > 0 {
			for range width {
				cells = append(cells, paintedCell{key(foreground), key(background), bold})
			}
			continue
		}
		if !strings.HasPrefix(seq, "\x1b[") || !strings.HasSuffix(seq, "m") {
			continue
		}
		params := p.Params()
		if len(params) == 0 {
			foreground, background, bold = nil, nil, false
		}
		for i := 0; i < len(params); i++ {
			value := params[i].Param(0)
			switch {
			case value == 0:
				foreground, background, bold = nil, nil, false
			case value == 1:
				bold = true
			case value == 22:
				bold = false
			case value == 39:
				foreground = nil
			case value == 49:
				background = nil
			case value == 38 || value == 48 || value == 58:
				var c color.Color
				if count := ansi.ReadStyleColor(params[i:], &c); count > 0 {
					if value == 38 {
						foreground = c
					}
					if value == 48 {
						background = c
					}
					i += count - 1
				}
			case value >= 30 && value <= 37:
				foreground = ansi.BasicColor(value - 30)
			case value >= 40 && value <= 47:
				background = ansi.BasicColor(value - 40)
			case value >= 90 && value <= 97:
				foreground = ansi.BasicColor(value - 90 + 8)
			case value >= 100 && value <= 107:
				background = ansi.BasicColor(value - 100 + 8)
			}
		}
	}
	return cells
}

func expectedPaint(foreground, background lipgloss.Color, bold bool) paintedCell {
	style := lipgloss.NewStyle().Foreground(foreground).Bold(bold)
	if background != "" {
		style = style.Background(background)
	}
	return paintedCells(style.Render("x"))[0]
}

func assertPaint(t *testing.T, text, subject string, foreground, background lipgloss.Color, bold bool) {
	t.Helper()
	plain := ansi.Strip(text)
	index := strings.Index(plain, subject)
	if index < 0 {
		t.Fatalf("missing %q in %q", subject, plain)
	}
	start, width := ansi.StringWidth(plain[:index]), ansi.StringWidth(subject)
	want := expectedPaint(foreground, background, bold)
	cells := paintedCells(text)
	for column, got := range cells[start : start+width] {
		if got != want {
			t.Fatalf("%q column %d: paint %+v, want %+v", subject, column, got, want)
		}
	}
}

// Every role differs from the default and from its neighbors. This is a test
// fixture for wiring, not a proposed user-facing theme or contrast guarantee.
func alternatePalette() Palette {
	var p Palette
	fields := reflect.ValueOf(&p).Elem()
	for i := 0; i < fields.NumField(); i++ {
		fields.Field(i).SetString(fmt.Sprintf("#%02x%02x%02x", 31+i*7, 213-i*5, 53+i*3))
	}
	return p
}

func TestFeedbackRolesUseInstancePaletteAndContinuousSurface(t *testing.T) {
	profile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.Ascii} {
		lipgloss.SetColorProfile(profile)
		for _, p := range []Palette{DefaultPalette(), alternatePalette()} {
			for _, size := range [][2]int{{60, 18}, {120, 32}} {
				for _, tc := range []struct {
					kind       feedbackKind
					foreground lipgloss.Color
					bold       bool
				}{
					{feedbackInfo, p.FooterInfo, true}, {feedbackSuccess, p.FooterSuccess, true}, {feedbackWarning, p.FooterWarning, true}, {feedbackError, p.FooterError, true}, {feedbackMuted, p.FooterMuted, false},
				} {
					m := resizeModel(demoModel().WithPalette(p), size[0], size[1])
					m.setToast(tc.kind, "Same wording")
					rows := strings.Split(m.View(), "\n")
					row := rows[len(rows)-1]
					assertPaint(t, row, "Same wording", tc.foreground, p.FooterBackground, tc.bold)
					assertPaint(t, row, " · ", p.FooterSeparator, p.FooterBackground, false)
					assertPaint(t, row, "^E", p.FooterHotkey, p.FooterBackground, true)
					want := expectedPaint(p.FooterText, p.FooterBackground, false).background
					cells := paintedCells(row)
					if len(cells) != size[0] {
						t.Fatal("footer changed dimensions")
					}
					for _, cell := range cells {
						if cell.background != want {
							t.Fatal("footer background gap", cell)
						}
					}
					if profile == termenv.Ascii && strings.Contains(row, "\x1b") {
						t.Fatal("color escapes in no-color footer")
					}
				}
				m := resizeModel(noticeModel(t).WithPalette(p), size[0], size[1])
				m.configStorageError, m.noticeStorageError = "Config save failed", "Could not save notices"
				assertPaint(t, m.View(), "2 errors", p.FooterWarning, p.FooterBackground, true)
				m = key(m, "enter")
				assertPaint(t, m.View(), "2 errors", p.FooterError, p.FooterBackground, true)
			}
		}
	}
}

func TestPaletteRebuildsCachedViewsWithoutLeakingOrChangingState(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	p := alternatePalette()
	oldColors := map[string]bool{}
	defaults := reflect.ValueOf(DefaultPalette())
	for i := 0; i < defaults.NumField(); i++ {
		oldColors[expectedPaint(defaults.Field(i).Interface().(lipgloss.Color), "", false).foreground] = true
	}
	for _, screen := range []string{"dashboard", "help", "inspector", "filament", "decisions", "attention", "nested alerts", "activity", "Config", "export", "status"} {
		t.Run(screen, func(t *testing.T) {
			m := resizeModel(noticeModel(t), 60, 18)
			switch screen {
			case "help":
				m = key(m, "?")
				m = key(m, "pgdown")
			case "inspector":
				m = key(m, "enter")
				m = key(m, "pgdown")
			case "filament":
				m = resizeModel(key(filamentModel(), "m"), 60, 18)
				m = key(m, "pgdown")
			case "decisions":
				m = key(key(m, "a"), "n")
				m = key(m, "pgdown")
			case "attention", "nested alerts":
				m = resizeModel(attentionTick(attentionReport(t, demoModel(), "H", runoutReport)), 60, 18)
				if screen == "nested alerts" {
					m = key(m, "s")
				}
			case "activity":
				m, _ = activityModel(20)
				m = resizeModel(key(m, "N"), 60, 18)
			case "Config":
				m = key(m, ",")
				m.configSelection = 1
				m = key(m, "enter")
			case "export":
				m = key(m, "D")
				m = key(m, "pgdown")
			case "status":
				m = key(m, "ctrl+e")
			}
			m.setToast(feedbackInfo, "This TUI owns desktop delivery")
			m.syncDialogs()
			before, view := hiddenState(m), m.View()
			editing, input, revision := m.configEditing, m.configInput, m.configRevision
			offsets := []int{m.details.YOffset, m.notices.YOffset, m.inspection.YOffset, m.help.YOffset, m.status.YOffset}
			if m.filament != nil {
				offsets = append(offsets, m.filament.view.YOffset)
			}
			alt := m.WithPalette(p)
			if !reflect.DeepEqual(before, hiddenState(alt)) || editing != alt.configEditing || input != alt.configInput || revision != alt.configRevision || m.toastAt != alt.toastAt || m.toastKind != alt.toastKind {
				t.Fatal("palette changed behavioral state")
			}
			after := []int{alt.details.YOffset, alt.notices.YOffset, alt.inspection.YOffset, alt.help.YOffset, alt.status.YOffset}
			if alt.filament != nil {
				after = append(after, alt.filament.view.YOffset)
			}
			if !reflect.DeepEqual(offsets, after) {
				t.Fatal("palette changed scroll", offsets, after)
			}
			if m.View() != view {
				t.Fatal("alternate palette leaked into the original model's cache")
			}
			for _, cell := range paintedCells(alt.View()) {
				if oldColors[cell.foreground] || oldColors[cell.background] {
					t.Fatal("cached default color survived palette change", cell)
				}
			}
			if got := alt.WithPalette(DefaultPalette()).View(); got != view {
				t.Fatal("default palette round trip changed rendering/state")
			}
		})
	}
}

func TestPaletteRolesReachFieldsFocusSelectionAndFilament(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	p := alternatePalette()
	m := filamentModel().WithPalette(p)
	assertPaint(t, m.detailText(100), "Printer Name:", p.SecondaryText, "", false)
	assertPaint(t, m.detailText(100), "H2D", p.Accent, "", true)
	assertPaint(t, m.section("PRINTERS", true, 100), "PRINTERS", p.Focus, "", false)
	assertPaint(t, m.printerRow(0), "H2D", p.SelectedText, p.SelectedBackground, false)
	assertPaint(t, m.printerRow(1), "X2D", p.Text, "", false)
	assertPaint(t, m.slotValue(m.current().AMS[2].Slots[1]), "Empty", p.MutedText, "", false)
	assertPaint(t, m.slotValue(m.current().AMS[2].Slots[2]), "●", lipgloss.Color("#918669"), "", false)
	assertPaint(t, m.headerHints("Enter edit"), "Enter", p.Hotkey, "", true)
	assertPaint(t, m.activityOutcome(notifications.Activity{Outcome: notifications.OutcomeAccepted}), "Accepted", p.Success, "", false)
	assertPaint(t, m.activityOutcome(notifications.Activity{Outcome: notifications.OutcomeFailed}), "Failed", p.Error, "", false)
	assertPaint(t, m.activityOutcome(notifications.Activity{Outcome: notifications.OutcomeUnconfirmed}), "Unconfirmed", p.Warning, "", false)
	m = key(m, "m")
	assertPaint(t, m.View(), "┌", p.Border, p.Background, false)
	m = key(m, "esc")
	m.setToast(feedbackInfo, "Palette information")
	m = key(m, "ctrl+e")
	assertPaint(t, m.View(), "Palette information", p.Info, p.Background, true)
}

func TestNotificationMessageKindsAndLocalFeedbackPriority(t *testing.T) {
	p := alternatePalette()
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	for _, tc := range []struct {
		kind       notifications.MessageKind
		want       feedbackKind
		foreground lipgloss.Color
	}{
		{notifications.MessageInfo, feedbackInfo, p.FooterInfo}, {notifications.MessageSuccess, feedbackSuccess, p.FooterSuccess}, {notifications.MessageWarning, feedbackWarning, p.FooterWarning}, {notifications.MessageError, feedbackError, p.FooterError},
	} {
		m, client, _ := notificationModel(t)
		m = m.WithPalette(p)
		client.status = notifications.Status{Revision: 1, Last: "This TUI owns desktop delivery", LastKind: tc.kind, Ownership: notifications.OwnershipOwner}
		m = persistenceUpdate(m, tickMsg(m.now))
		if m.toastKind != tc.want {
			t.Fatal("notification meaning was lost")
		}
		assertPaint(t, m.View(), "This TUI owns desktop delivery", tc.foreground, p.FooterBackground, true)
		m.setToast(feedbackSuccess, "Local decision updated; printer state unchanged")
		at := m.toastAt
		client.status.Revision++
		client.status.LastKind = notifications.MessageInfo
		m = persistenceUpdate(m, tickMsg(m.now.Add(time.Second)))
		if m.toastKind != feedbackSuccess || m.toastAt != at || !strings.HasPrefix(m.toast, "Local decision") {
			t.Fatal("background update replaced local feedback meaning/lifetime")
		}
		m = persistenceUpdate(m, tickMsg(at.Add(toastLifetime+time.Second)))
		if m.toast != "" {
			t.Fatal("ordinary feedback lifetime changed")
		}
	}
}

func TestConfigTestMeaningAndDimmingAreIndependentOfPalette(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	p := alternatePalette()
	for _, failure := range []bool{false, true} {
		m := openAlertConfig(demoModel().WithPalette(p).WithNotificationTest(context.Background(), func(context.Context, bool) error {
			if failure {
				return errors.New("unavailable")
			}
			return nil
		}))
		cmd := m.startNotificationTest()
		assertPaint(t, m.configFooter(), "Test: Sending", p.FooterInfo, p.FooterBackground, true)
		next, _ := m.Update(cmd())
		m = next.(Model)
		if failure {
			m.now = m.now.Add(time.Hour)
			assertPaint(t, m.configFooter(), "Test failed", p.FooterError, p.FooterBackground, true)
			continue
		}
		assertPaint(t, m.configFooter(), "Test accepted", p.FooterSuccess, p.FooterBackground, true)
		m.now = m.notificationTestStatusAt.Add(notificationTestDimAfter)
		assertPaint(t, m.configFooter(), "Test accepted", p.FooterMuted, p.FooterBackground, false)
		m.now = m.notificationTestStatusAt.Add(notificationTestFeedbackLifetime)
		if strings.Contains(ansi.Strip(m.configFooter()), "Test accepted") {
			t.Fatal("test feedback expiry changed")
		}
	}
}

func TestApplicationColorLiteralsBelongToPalette(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	hex := regexp.MustCompile(`"#[[:xdigit:]]{6}"`)
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if hex.Match(data) {
			t.Fatal("application color literal outside the JSON palette definitions", name)
		}
	}
}

func TestBundledPaletteContrast(t *testing.T) {
	luminance := func(c lipgloss.Color) float64 {
		hex := string(c)
		if len(hex) != 7 || hex[0] != '#' {
			t.Fatal("bundled palettes require explicit RGB for measured pairs", c)
		}
		rgb, err := strconv.ParseUint(hex[1:], 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		var result float64
		for i, weight := range []float64{0.2126, 0.7152, 0.0722} {
			channel := float64((rgb>>(16-i*8))&255) / 255
			if channel <= 0.04045 {
				channel /= 12.92
			} else {
				channel = math.Pow((channel+0.055)/1.055, 2.4)
			}
			result += channel * weight
		}
		return result
	}
	contrast := func(a, b lipgloss.Color) float64 {
		x, y := luminance(a), luminance(b)
		return (max(x, y) + 0.05) / (min(x, y) + 0.05)
	}
	for _, name := range []string{"default", "gruvbox-dark", "hotdog"} {
		theme, err := LoadThemeOverride(name)
		if err != nil {
			t.Fatal(err)
		}
		p := theme.Palette
		for _, c := range []lipgloss.Color{p.FooterInfo, p.FooterSuccess, p.FooterWarning, p.FooterError, p.FooterText, p.FooterHotkey, p.FooterSeparator} {
			if ratio := contrast(c, p.FooterBackground); ratio < 4.5 {
				t.Fatalf("%s active footer role %s contrast %.2f, want at least 4.5", name, c, ratio)
			}
		}
		if contrast(p.FooterInfo, p.FooterBackground) < 7 || contrast(p.FooterInfo, p.FooterBackground) <= contrast(p.FooterMuted, p.FooterBackground) {
			t.Fatal(name, "fresh informational text lost prominence over deliberately dimmed feedback")
		}
		if name == "hotdog" {
			if luminance(p.Background) < 0.9 {
				t.Fatal("Hotdog must retain its very light main background")
			}
			for _, c := range []lipgloss.Color{p.Text, p.SecondaryText, p.MutedText, p.Info, p.Accent, p.Success, p.Warning, p.Error, p.Focus, p.Border, p.Hotkey} {
				if ratio := contrast(c, p.Background); ratio < 4.5 {
					t.Fatalf("Hotdog body role %s contrast %.2f, want at least 4.5", c, ratio)
				}
			}
			if contrast(p.SelectedText, p.SelectedBackground) < 4.5 {
				t.Fatal("Hotdog selection lost readable text")
			}
		}
	}
}
