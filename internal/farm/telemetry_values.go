package farm

// Packed-field layouts and sentinels follow the existing Bambu telemetry format.
// Limits bound accepted measurements; MaxRemainingMinutes is one year.
const (
	versionSuccess        = "success"
	activeNozzleShift     = 4
	nozzleIndexMask       = 0x0f
	slotIDMask            = 0x0f
	unitIDShift           = 8
	legacySlotBits        = 2
	legacySlotMask        = SlotsPerAMS - 1
	legacyExternalTray    = 254
	legacyNoTray          = 255
	spoolPresentMask      = 1
	spoolReadyState       = 3
	spoolMetadataMask     = 8
	temperatureBits       = 16
	temperatureMask       = 0xffff
	maxAMSTemperature     = 100
	emptyMaterial         = "Empty"
	commandStatus         = "push_status"
	commandVersion        = "get_version"
	otaModule             = "ota"
	upgradeAvailable      = "1"
	upgradeCurrent        = "2"
	firmwarePartCount     = 4
	maxFirmwarePartDigits = 4
	firmwareUpgradeIdle   = "IDLE"
)
