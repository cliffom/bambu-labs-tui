package ui

import "time"

// UI policies are independent even where durations currently match.
const (
	firmwareMaxAge        = 2 * time.Minute
	attentionDeferral     = 2 * time.Second
	shutdownSaveTimeout   = 5 * time.Second
	toastLifetime         = 8 * time.Second
	browserTimeout        = 10 * time.Second
	dashboardTickInterval = time.Second
	minimumWidth          = 60
	minimumHeight         = 18
	sideBySideWidth       = 105
	compactTableWidth     = 80
	fullTableWidth        = 110
	progressBarWidth      = 8
	minutesPerHour        = 60
	aboutPaletteCellWidth = 8
	aboutSwatchWidth      = 6
	dialogInset           = 8
	dialogBorderInset     = 4
	colorRGBDigits        = 6
	colorRGBADigits       = 8
	hexDigits             = "0123456789ABCDEF"
)

const (
	notificationTestDimAfter         = 4 * time.Second
	notificationTestFeedbackLifetime = 7 * time.Second
)

const (
	snoozeShort              = 15 * time.Minute
	snoozeHour               = time.Hour
	snoozeLong               = 4 * time.Hour
	defaultWidth             = 100
	defaultHeight            = 32
	panelGap                 = 3
	detailsWidthPercent      = 57
	dashboardReservedRows    = 11
	minimumPanelRows         = 4
	maximumPrinterRows       = 6
	printerRowsHeightDivisor = 5
	estimatedNoticeRows      = 3
)

const (
	noticeTimestampLayout = "2006-01-02 15:04:05 MST"
	versionLabelMaxWidth  = 24
)

const (
	activeNoticePrefix     = "active:"
	eventNoticeKeyFormat   = "event:%d"
	amsUnitKeyFormat       = "ams:%d"
	externalUnitKey        = "external"
	filamentSlotWidth      = 14
	filamentRemainingWidth = 10
	filamentNozzleWidth    = 10
)

// Startup dimensions are replaced by the owning panel/dialog sync method.
const (
	initialDetailsWidth    = 50
	initialDetailsHeight   = 15
	initialNoticesWidth    = 40
	initialNoticesHeight   = 15
	initialInspectorWidth  = 90
	initialInspectorHeight = 26
)
