package diagnostics

// Diagnostic schema/status vocabulary and independent sanitization limits.
// Allowlist field names and enum members remain explicit declarative schema data.
const (
	maxByteValue              = 255
	privateDirectoryMode      = 0o700
	privateFileMode           = 0o600
	schemaVersion             = 1
	sanitizeNodeBudget        = 512
	statusInvalidJSON         = "invalid_json"
	statusTooLarge            = "too_large"
	statusSanitized           = "sanitized"
	statusUnsupportedEnvelope = "unsupported_envelope"
	typeUnavailable           = "unavailable"
	typeNull                  = "null"
	typeObject                = "object"
	typeArray                 = "array"
	typeString                = "string"
	typeNumber                = "number"
	typeBoolean               = "boolean"
	typeUnsupported           = "unsupported"
	ruleEnum                  = "enum"
	rulePattern               = "pattern"
	minTemperature            = -50
	maxTemperature            = 500
	maxHMSItems               = 64
	maxAMSItems               = 16
	maxModules                = 16
	maxUpgradeModules         = 8
	nozzleCount               = 2
	maxHumidityLevel          = 5
)
