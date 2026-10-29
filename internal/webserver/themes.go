package webserver

import "encoding/json"

type (
	// Theme contains only an opaque choice, a public label, and resolved RGB roles.
	// Paths and arbitrary theme-file metadata never cross the HTTP boundary.
	Theme struct {
		ID     string            `json:"id"`
		Label  string            `json:"label"`
		Colors map[string]string `json:"colors"`
	}
	ThemeCatalog struct {
		Themes []Theme `json:"themes"`
	}
)

// PublishThemes detaches the catalog from its producer before HTTP readers see it.
func (s *Server) PublishThemes(catalog ThemeCatalog) {
	data, err := json.Marshal(catalog)
	if err != nil {
		panic(err)
	}
	s.themes.Store(data)
}
