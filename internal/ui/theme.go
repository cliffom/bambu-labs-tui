package ui

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
)

//go:embed themes/default.json
var defaultTheme []byte

//go:embed themes/gruvbox-dark.json
var gruvboxDarkTheme []byte

const maximumThemeBytes = 64 * 1024

var themeRGB = regexp.MustCompile(`^#[[:xdigit:]]{6}$`)

// Theme is a fully resolved value. Files may override any subset of colors.
type Theme struct {
	Name    string
	Palette Palette
}

// LoadThemeOverride accepts a bundled name or a file relative to the working
// directory. This is a launch-only CLI choice; configuration paths are separate.
func LoadThemeOverride(choice string) (Theme, error) {
	switch choice {
	case "default":
		return ParseTheme(defaultTheme)
	case "gruvbox-dark":
		return ParseTheme(gruvboxDarkTheme)
	case "":
		return Theme{}, fmt.Errorf("--theme requires default, gruvbox-dark, or a JSON file path")
	default:
		return LoadTheme("", choice)
	}
}

// LoadTheme resolves relative files beside the active TOML configuration.
// An omitted path uses the embedded default, with no filesystem dependency.
func LoadTheme(configPath, themeFile string) (Theme, error) {
	if themeFile == "" {
		return ParseTheme(defaultTheme)
	}
	path := themeFile
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(configPath), path)
	}
	f, err := os.Open(path)
	if err != nil {
		return Theme{}, fmt.Errorf("read theme %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maximumThemeBytes+1))
	if err != nil {
		return Theme{}, fmt.Errorf("read theme %q: %w", path, err)
	}
	theme, err := ParseTheme(data)
	if err != nil {
		return Theme{}, fmt.Errorf("theme %q: %w", path, err)
	}
	return theme, nil
}

// ParseTheme accepts version 1 JSON only. Reject ambiguous/unknown keys rather
// than silently losing a customization. Null is meaningful only for Background.
func ParseTheme(data []byte) (Theme, error) {
	if len(data) > maximumThemeBytes {
		return Theme{}, fmt.Errorf("theme exceeds %d bytes", maximumThemeBytes)
	}
	fields, err := themeObject(data)
	if err != nil {
		return Theme{}, err
	}
	for key := range fields {
		if key != "version" && key != "name" && key != "colors" {
			return Theme{}, fmt.Errorf("unknown theme key %q", key)
		}
	}
	var version int
	if json.Unmarshal(fields["version"], &version) != nil || version != 1 {
		return Theme{}, fmt.Errorf("theme version must be 1")
	}
	var theme Theme
	if raw, ok := fields["name"]; ok {
		if bytes.Equal(raw, []byte("null")) || json.Unmarshal(raw, &theme.Name) != nil {
			return Theme{}, fmt.Errorf("theme name must be a string")
		}
	}
	colors, err := themeObject(fields["colors"])
	if err != nil {
		return Theme{}, fmt.Errorf("theme colors: %w", err)
	}
	// Decode the complete embedded document directly to avoid recursive defaults.
	var defaults struct {
		Colors Palette `json:"colors"`
	}
	if err := json.Unmarshal(defaultTheme, &defaults); err != nil {
		return Theme{}, err
	}
	theme.Palette = defaults.Colors
	value := reflect.ValueOf(&theme.Palette).Elem()
	roles := make(map[string]int, value.NumField())
	for i := 0; i < value.NumField(); i++ {
		roles[value.Type().Field(i).Tag.Get("json")] = i
	}
	for role, raw := range colors {
		i, ok := roles[role]
		if !ok {
			return Theme{}, fmt.Errorf("unknown theme color %q", role)
		}
		if role == "background" && bytes.Equal(raw, []byte("null")) {
			value.Field(i).SetString("")
			continue
		}
		var color string
		if json.Unmarshal(raw, &color) != nil || !themeRGB.MatchString(color) {
			return Theme{}, fmt.Errorf("theme color %q must be #RRGGBB%s", role, backgroundNullHint(role))
		}
		value.Field(i).SetString(color)
	}
	return theme, nil
}

func backgroundNullHint(role string) string {
	if role == "background" {
		return " or null"
	}
	return ""
}

// RawMessage preserves duplicate detection at both schema levels. The decoder
// alone otherwise accepts duplicate keys and case-insensitive struct fields.
func themeObject(data []byte) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("expected a JSON object")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("expected an object key")
		}
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("duplicate theme key %q", key)
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, err
		}
		fields[key] = raw
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after JSON object")
	}
	return fields, nil
}
