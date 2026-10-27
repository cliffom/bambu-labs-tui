package farm

import "time"

// Shared domain values retain their wire spelling, including unknown-state fallbacks.
// Time policies remain separate even when their current durations match.
const (
	StateRunning                = "RUNNING"
	StatePreparing              = "PREPARE"
	StateSlicing                = "SLICING"
	StatePaused                 = "PAUSE"
	StateFailed                 = "FAILED"
	StateFinished               = "FINISH"
	StateIdle                   = "IDLE"
	StateUnknown                = "UNKNOWN"
	LevelError                  = "error"
	LevelWarning                = "warning"
	LevelInfo                   = "info"
	AlertFilament               = "filament"
	AlertExtrusion              = "extrusion"
	AlertPrinterFaults          = "printer_faults"
	AlertFailedPrints           = "failed_prints"
	AlertUnknownPauses          = "unknown_pauses"
	ConnectionCloudDisconnected = "Cloud disconnected"
	ConnectionOffline           = "Offline"
	ConnectionWaiting           = "Waiting for data"
	ConnectionStale             = "Stale"
	ConnectionLive              = "Live"
	activeTelemetryMaxAge       = 2 * time.Minute
	remainingTelemetryMaxAge    = 2 * time.Minute
	idleTelemetryMaxAge         = 10 * time.Minute
	MaxEventHistory             = 500
	MaxPercent                  = 100
	MaxRemainingMinutes         = 365 * 24 * 60
	MaxLayers                   = 1_000_000
	SlotsPerAMS                 = 4
	RightNozzle                 = 0
	LeftNozzle                  = 1
	AMSHTFirstID                = 128
	AMSHTLastID                 = 135
	LegacyExternalSpoolID       = 255
	ExternalSpoolFirstID        = 254
	ExternalSpoolNumberBase     = 256
)
