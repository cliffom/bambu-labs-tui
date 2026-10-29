package webserver

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestPublishedThemesDetachFromProducerAndReplaceAtomically(t *testing.T) {
	s := New(t.Context())
	defer s.Close()
	catalog := ThemeCatalog{Themes: []Theme{{ID: "host", Label: "Follow host", Colors: map[string]string{"text": "#ABCDEF"}}}}
	s.PublishThemes(catalog)
	catalog.Themes[0].Colors["text"] = "#123456"
	handler := s.handler("127.0.0.1:8080", nil)
	read := func() ThemeCatalog {
		t.Helper()
		request := httptest.NewRequest("GET", "http://127.0.0.1:8080/api/themes", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		var result ThemeCatalog
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if got := read().Themes[0].Colors["text"]; got != "#ABCDEF" {
		t.Fatal("producer mutated published catalog", got)
	}
	s.PublishThemes(catalog)
	if got := read().Themes[0].Colors["text"]; got != "#123456" {
		t.Fatal("new palette not visible", got)
	}
}
