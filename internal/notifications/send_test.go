package notifications

import (
	"strings"
	"testing"
)

func TestNotificationBodyIsBoundedAndPlain(t *testing.T) {
	body := cleanBody("Printer\n\x00\x1b" + strings.Repeat("印", 300))
	if len([]rune(body)) != maxNotificationBodyRunes || strings.ContainsAny(body, "\n\x00\x1b") {
		t.Fatal("invalid notification text")
	}
}

func TestNotificationTitleIsPlainBoundedAndHasFallback(t *testing.T) {
	if title := cleanTitle("\nX2D\x00\x1b"); title != "X2D" {
		t.Fatal(title)
	}
	if title := cleanTitle(strings.Repeat("印", 200)); len([]rune(title)) != maxNotificationTitleRunes {
		t.Fatal("unbounded title")
	}
	if title := cleanTitle("\n\x00 "); title != "Bambu TUI" {
		t.Fatal("missing safe fallback", title)
	}
}
