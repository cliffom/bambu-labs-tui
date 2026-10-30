package config

// Configuration identifiers and defaults; TOML field tags stay literal.
const (
	DefaultWebPort            = 8080
	RegionGlobal              = "global"
	RegionChina               = "china"
	defaultLowFilamentPercent = 10
	maxPercent                = 100
	// Retain the original namespace for upgrade compatibility; see docs/user/usage.md.
	applicationDirectory = "bambu-tui"
	configFilename       = "config.toml"
)
