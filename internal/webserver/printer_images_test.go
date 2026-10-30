package webserver

import (
	"crypto/sha256"
	"fmt"
	"image/png"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrinterModelImagesAndUnknownFallback(t *testing.T) {
	for _, test := range []struct{ model, key string }{
		{"H2D", "h2d"},
		{"Bambu Lab X2D", "x2d"},
		{" x1-carbon ", "x1c"},
		{"X1C", "x1c"},
		{"Bambu Lab P1S", "p1s"},
		{"A1 mini", "a1mini"},
		{"a1_mini", "a1mini"},
		{"", ""},
		{"Unknown", ""},
		{"H2D Pro", ""},
		{"X1E", ""},
		{"A1", ""},
		{"P1P", ""},
		{"../../H2D", ""},
		{"<script>H2D</script>", ""},
	} {
		got := printerImage(test.model)
		want := ""
		if test.key != "" {
			want = printerImages[test.key]
		}
		if got != want {
			t.Fatalf("model %q: %q, want %q", test.model, got, want)
		}
	}
	state, now := syntheticState(t)
	if got := decodeSnapshot(t, snapshotJSON(state, now, true)).Printers[0].Image; got != printerImages["h2d"] {
		t.Fatal("snapshot omitted known model artwork", got)
	}
	p := state.Printers["PRIVATE-SERIAL"]
	p.Name = "H2D"
	p.Model = "Unknown"
	p.ModelCode = ""
	if got := decodeSnapshot(t, snapshotJSON(state, now, true)).Printers[0].Image; got != "" {
		t.Fatal("user-assigned name selected model artwork")
	}
}

func TestPrinterSpritesAreEmbeddedTransparentAndCacheVersioned(t *testing.T) {
	server := New(t.Context())
	t.Cleanup(server.Close)
	// Use the real fixed-path HTTP handler, without opening an extra listener.
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		t.Fatal(err)
	}
	handler := server.handler("127.0.0.1:12345", http.FileServer(http.FS(sub)))
	for _, path := range printerImages {
		request := httptest.NewRequest("GET", "http://127.0.0.1:12345"+path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 200 || response.Header().Get("Content-Type") != "image/png" {
			t.Fatal(path, response.Code, response.Header())
		}
		if !strings.Contains(response.Header().Get("Cache-Control"), "immutable") {
			t.Fatal("versioned art is not cached")
		}
		image, err := png.Decode(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		bounds := image.Bounds()
		_, _, _, alpha := image.At(bounds.Min.X, bounds.Min.Y).RGBA()
		if bounds.Dx() < 1 || bounds.Dy() < 1 || alpha != 0 {
			t.Fatal("missing dimensions or transparent padding", path)
		}
		data, err := assets.ReadFile("assets" + path)
		if err != nil {
			t.Fatal(err)
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(data))
		if !strings.Contains(path, digest[:12]) {
			t.Fatal("cache version does not identify embedded bytes", path)
		}
	}
	for _, path := range []string{"/printers/", "/printers/h2d.png", "/printers/unknown.png"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "http://127.0.0.1:12345"+path, nil))
		if response.Code != 404 {
			t.Fatal("unlisted asset route exposed", path)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "http://127.0.0.1:12345/api/snapshot", nil))
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("artwork cache policy leaked to telemetry")
	}
}
