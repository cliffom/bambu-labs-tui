package ui

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

//go:embed themes/*.json
var bundledThemes embed.FS

const maximumThemeBytes = 64 * 1024

var themeRGB = regexp.MustCompile(`^#[[:xdigit:]]{6}$`)

// Theme is a fully resolved value. Files may override any subset of colors.
type Theme struct {
	Name    string
	Source  string // Bundled name or resolved custom-file path; independent of JSON name.
	Palette Palette
}

// BundledThemeNames discovers names from JSON filenames, in sorted order.
// Adding a shipped theme requires only its JSON file, with no Go registration.
func BundledThemeNames() ([]string, error) {
	return bundledThemeNames(bundledThemes)
}

func bundledThemeNames(source fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(source, "themes")
	if err != nil {
		return nil, fmt.Errorf("list bundled themes: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	return names, nil
}

// LoadThemeOverride accepts a bundled name or a file relative to the working
// directory. This is a launch-only CLI choice; configuration paths are separate.
func LoadThemeOverride(choice string) (Theme, error) {
	return loadThemeOverride(choice, bundledThemes)
}

func loadThemeOverride(choice string, source fs.FS) (Theme, error) {
	if choice == "" {
		return Theme{}, fmt.Errorf("--theme requires a bundled theme name or a JSON file path")
	}
	// Only bare names address bundled files. Explicit paths retain their working
	// directory semantics and cannot traverse the embedded theme directory.
	if fs.ValidPath(choice) && !strings.ContainsAny(choice, `/\`) {
		data, err := fs.ReadFile(source, "themes/"+choice+".json")
		if err == nil {
			theme, err := ParseTheme(data)
			theme.Source = choice
			return theme, err
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return Theme{}, fmt.Errorf("read bundled theme %q: %w", choice, err)
		}
	}
	return LoadTheme("", choice)
}

// LoadTheme resolves relative files beside the active TOML configuration.
// An omitted path uses the embedded default, with no filesystem dependency.
func LoadTheme(configPath, themeFile string) (Theme, error) {
	if themeFile == "" {
		data, err := bundledThemes.ReadFile("themes/default.json")
		if err != nil {
			return Theme{}, fmt.Errorf("read bundled default theme: %w", err)
		}
		theme, err := ParseTheme(data)
		theme.Source = "default"
		return theme, err
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
	theme.Source = path
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
	defaultData, err := bundledThemes.ReadFile("themes/default.json")
	if err != nil {
		return Theme{}, fmt.Errorf("read bundled default theme: %w", err)
	}
	if err := json.Unmarshal(defaultData, &defaults); err != nil {
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
