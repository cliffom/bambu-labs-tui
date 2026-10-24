package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Region             string `toml:"region"`
	LowFilamentPercent int    `toml:"low_filament_percent"`
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bambu-tui", "config.toml"), nil
}

func Load(path string) (Config, error) {
	c := Config{Region: "global", LowFilamentPercent: 10}
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
	if c.Region != "global" && c.Region != "china" {
		return c, fmt.Errorf("region must be global or china")
	}
	if c.LowFilamentPercent < 0 || c.LowFilamentPercent > 100 {
		return c, fmt.Errorf("low_filament_percent must be between 0 and 100")
	}
	return c, nil
}
