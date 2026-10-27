package ui

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// canvas paints the whole viewport, including blank rows and dialog margins.
// Nested styles retain their foreground/background; their resets restore the
// body surface instead of exposing the terminal's unrelated default colors.
func (s *styles) canvas(text string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(fit(text, width, height), "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		line += strings.Repeat(" ", max(0, width-ansi.StringWidth(line)))
		lines[i] = paintSurface(line, s.palette.Text, s.palette.Background)
	}
	return strings.Join(lines, "\n")
}

func surfaceColor(c lipgloss.Color, background bool) string {
	if c == "" {
		return ""
	}
	// Obtain the prefix through Lip Gloss so body defaults use exactly the
	// same RGB conversion, limited-color profile, and NO_COLOR behavior as roles.
	style := lipgloss.NewStyle()
	if background {
		style = style.Background(c)
	} else {
		style = style.Foreground(c)
	}
	prefix, _, _ := strings.Cut(style.Render("x"), "x")
	return prefix
}

func paintSurface(text string, foreground, background lipgloss.Color) string {
	fg, bg := surfaceColor(foreground, false), surfaceColor(background, true)
	if fg == "" && bg == "" {
		return text
	}
	var out strings.Builder
	out.WriteString(fg + bg)
	parser := ansi.NewParser()
	var state byte
	hasFG, hasBG := true, true
	for len(text) > 0 {
		seq, _, n, next := ansi.DecodeSequence(text, state, parser)
		text, state = text[n:], next
		out.WriteString(seq)
		if !strings.HasPrefix(seq, "\x1b[") || !strings.HasSuffix(seq, "m") {
			continue
		}
		params := parser.Params()
		if len(params) == 0 {
			hasFG, hasBG = false, false
		}
		for i := 0; i < len(params); i++ {
			v := params[i].Param(0)
			switch {
			case v == 0:
				hasFG, hasBG = false, false
			case v == 39:
				hasFG = false
			case v == 49:
				hasBG = false
			case v == 38 || v == 48 || v == 58:
				var c color.Color
				if count := ansi.ReadStyleColor(params[i:], &c); count > 0 {
					if v == 38 {
						hasFG = true
					}
					if v == 48 {
						hasBG = true
					}
					i += count - 1
				}
			case v >= 30 && v <= 37 || v >= 90 && v <= 97:
				hasFG = true
			case v >= 40 && v <= 47 || v >= 100 && v <= 107:
				hasBG = true
			}
		}
		if !hasFG {
			out.WriteString(fg)
			hasFG = true
		}
		if !hasBG {
			out.WriteString(bg)
			hasBG = true
		}
	}
	out.WriteString("\x1b[0m")
	return out.String()
}
