package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Region             string           `toml:"region"`
	LowFilamentPercent int              `toml:"low_filament_percent"`
	Alerts             AlertPreferences `toml:"alerts"`
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, applicationDirectory, configFilename), nil
}

func Load(path string) (Config, error) {
	c := Config{Region: RegionGlobal, LowFilamentPercent: defaultLowFilamentPercent, Alerts: DefaultAlerts()}
	meta, err := toml.DecodeFile(path, &c)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("read configuration: %w", err)
	}
	if len(meta.Undecoded()) != 0 {
		return c, fmt.Errorf("unknown configuration key: %s", meta.Undecoded()[0])
	}
	return c, c.Validate()
}

// Validate is shared by file loading and interactive editing.
func (c Config) Validate() error {
	if c.Region != RegionGlobal && c.Region != RegionChina {
		return fmt.Errorf("region must be global or china")
	}
	if c.LowFilamentPercent < 0 || c.LowFilamentPercent > maxPercent {
		return fmt.Errorf("low_filament_percent must be between 0 and 100")
	}
	for id := range c.Alerts.Printers {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("alert printer ID must not be empty")
		}
	}
	return nil
}
