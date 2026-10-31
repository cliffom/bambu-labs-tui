package config

import "github.com/cliffom/bambustand/internal/appidentity"

// Configuration identifiers and defaults; TOML field tags stay literal.
const (
	DefaultWebPort            = 8080
	RegionGlobal              = "global"
	RegionChina               = "china"
	defaultLowFilamentPercent = 10
	maxPercent                = 100
	applicationDirectory      = appidentity.Namespace
	configFilename            = "config.toml"
)
