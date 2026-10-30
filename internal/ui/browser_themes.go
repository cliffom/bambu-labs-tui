package ui

import (
	"reflect"

	"github.com/cliffom/bambustand/internal/webserver"
)

// BrowserThemes reuses the terminal inventory and effective palette. Only fixed
// labels and validated RGB roles leave the host; custom paths/names stay private.
func BrowserThemes(p Palette) webserver.ThemeCatalog {
	convert := func(id, label string, palette Palette) webserver.Theme {
		colors := make(map[string]string)
		value := reflect.ValueOf(palette)
		for i := 0; i < value.NumField(); i++ {
			color := value.Field(i).String()
			if themeRGB.MatchString(color) {
				colors[value.Type().Field(i).Tag.Get("json")] = color
			}
		}
		return webserver.Theme{ID: id, Label: label, Colors: colors}
	}
	catalog := webserver.ThemeCatalog{Themes: []webserver.Theme{convert("host", "Follow host (configured theme)", p)}}
	names, err := BundledThemeNames()
	if err != nil {
		panic(err)
	}
	for _, name := range names {
		theme, err := LoadThemeOverride(name)
		if err != nil {
			panic(err)
		}
		catalog.Themes = append(catalog.Themes, convert(name, theme.Name, theme.Palette))
	}
	return catalog
}

func (m Model) publishBrowserThemes() {
	if server, ok := m.webServer.(interface{ PublishThemes(webserver.ThemeCatalog) }); ok {
		server.PublishThemes(BrowserThemes(m.styles.palette))
	}
}
