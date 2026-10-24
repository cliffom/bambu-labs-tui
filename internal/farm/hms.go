package farm

import "fmt"

// Bambu Studio's DevHMS encodes the message level in the upper word of code:
// 1=fatal, 2=serious, 3=common, 4=info. Keep unknown levels visible.
func hmsLevel(code int) string {
	switch uint32(code) >> 16 {
	case 1, 2:
		return "error"
	case 3:
		return "warning"
	case 4:
		return "info"
	default:
		return "warning"
	}
}

// Concise descriptions are independently phrased from Bambu Studio's English
// HMS catalogues. A known empty catalogue entry is internal and is suppressed.
// See docs/protocol-sources.md for the source files and fallback policy.
var hmsMessages = buildHMSMessages()

func buildHMSMessages() map[string]string {
	messages := map[string]string{
		"0500_0200_0003_000A": "",
		"0500_0300_0002_0055": "Printer account information has expired; check its Bambu account connection",
		"0501_0400_0003_0002": "Maintenance due: lubricate the threaded rods",
		"0501_0400_0003_0004": "Maintenance due: clean and lubricate the motion-system rails and rods",
		"0300_0100_0001_0007": "Heatbed temperature sensor may have an open circuit",
		"0500_0100_0003_0004": "Printer removable storage is low on free space",
		"0500_0400_0002_0043": "Nozzle camera needs cleaning or is obstructed",
		"0500_0600_0002_0032": "Nozzle camera is disconnected; check its cable connection",
		"0500_0300_0001_0023": "Chamber temperature control is malfunctioning",
		"0500_0300_0001_0025": "Printer firmware is abnormal; check the firmware installation",
		"0500_0400_0002_0007": "Bed temperature may cause a nozzle clog; check cooling and filament settings",
	}
	addFilamentHMS(messages)
	return messages
}

// AMS HT units use IDs 128..135, independently of conventional AMS IDs.
// Labels are separate from transport IDs so nozzle/slot assignments stay intact.
func AMSLabel(id int) string {
	if id >= 128 && id <= 135 {
		return fmt.Sprintf("AMS HT %d", id-127)
	}
	return fmt.Sprintf("AMS %d", id+1)
}
