package webserver

import "strings"

// Content-versioned local assets are decorative hardware depictions, never
// telemetry. Only known model names resolve; unknown models keep the fallback.
var printerImages = map[string]string{
	"a1mini": "/printers/a1-mini-99cd4423fabe.png",
	"h2d":    "/printers/h2d-ab6291473de5.png",
	"p1s":    "/printers/p1s-1cf611b647f6.png",
	"x1c":    "/printers/x1c-72b4402798e9.png",
	"x2d":    "/printers/x2d-b7f68caf0fdf.png",
}

func printerImage(model string) string {
	model = strings.NewReplacer(" ", "", "-", "", "_", "").Replace(strings.ToLower(strings.TrimSpace(model)))
	model = strings.TrimPrefix(model, "bambulab")
	if model == "x1carbon" {
		model = "x1c"
	}
	return printerImages[model]
}

func isPrinterImage(path string) bool {
	for _, image := range printerImages {
		if path == image {
			return true
		}
	}
	return false
}
