package config

// Configuration identifiers and defaults; TOML field tags stay literal.
const (
	RegionGlobal              = "global"
	RegionChina               = "china"
	defaultLowFilamentPercent = 10
	maxPercent                = 100
	applicationDirectory      = "bambu-tui"
	configFilename            = "config.toml"
)
