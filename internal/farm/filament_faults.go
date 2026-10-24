package farm

import "fmt"

// These finite code families were checked against the pinned Bambu Studio
// catalogues in docs/protocol-sources.md. Do not extrapolate new units, slots,
// or subcodes: unknown codes must retain the normal visible fallback.
func addFilamentHMS(messages map[string]string) {
	// The high word is the component (0x1000, 0x2000, ...); low words
	// are the HMS severity and error identifier, exactly as printed in codes.
	slotMessages := map[uint64]string{
		0x1000_0002_0002: "feeder motor overloaded; filament may be tangled or stuck",
		0x2000_0002_0001: "filament runout; replace the spool in this slot",
		0x2000_0002_0002: "slot empty; load filament before continuing",
		0x2000_0002_0003: "filament may have broken inside the AMS",
		0x2000_0002_0004: "filament may have broken inside the toolhead",
		0x2000_0002_0005: "runout recovery failed; check for filament stuck in the toolhead",
		0x2000_0002_0009: "extrusion failed; possible clog or slipping filament",
		0x2000_0002_0010: "filament feed timed out at the AMS",
		0x2000_0002_0011: "filament retraction timed out at the AMS",
		0x2000_0002_0012: "feeder stalled; the spool cannot turn",
		0x2000_0002_0024: "spool could not rewind during retraction",
		0x2000_0003_0001: "filament runout; automatic purging is in progress",
		0x2000_0003_0002: "filament runout; automatic backup-spool switch reported",
		0x6000_0002_0001: "filament or buffer may be jammed or tangled",
		0x7000_0002_0001: "filament cannot leave the extruder; possible clog or broken filament",
		0x7000_0002_0002: "filament cannot reach the toolhead; check the spool and filament path",
		0x7000_0002_0004: "filament cannot retract from the toolhead; check for a stuck spool or filament",
		0x7000_0002_0005: "filament feeding failed; check the filament end and spool movement",
	}
	for _, prefix := range []uint32{0x07, 0x18} {
		for unit := range 8 {
			name := filamentUnit(prefix, unit)
			for slot := range 4 {
				for family, text := range slotMessages {
					attr := prefix<<24 | uint32(unit)<<16 | uint32(family>>32) | uint32(slot)<<8
					messages[hmsCode(attr, uint32(family))] = fmt.Sprintf("%s slot %d: %s", name, slot+1, text)
				}
			}
			// These operation-level codes exist only at 0x7000, not 0x7100..0x7300.
			for code, text := range map[uint32]string{
				0x00020003: "extrusion failed; check for an extruder or nozzle clog",
				0x00020006: "purge timed out; filament may be stuck or the extruder clogged",
				0x00020007: "filament runout; replace filament in the same AMS slot",
			} {
				attr := prefix<<24 | uint32(unit)<<16 | 0x7000
				messages[hmsCode(attr, code)] = name + ": " + text
			}
		}
		for _, port := range []uint32{0xFE, 0xFF} {
			for code, text := range map[uint32]string{
				0x00020001: "External spool filament runout; load a replacement spool",
				0x00020002: "No external spool filament detected; load filament",
			} {
				messages[hmsCode(prefix<<24|port<<16|0x2000, code)] = text
			}
		}
	}
	for code, text := range map[string]string{
		"0300_0900_0002_0001": "Extruder motor overloaded; possible clog or filament jam",
		"0300_0900_0002_0002": "Extrusion resistance abnormal; possible clog or filament jam",
		"0300_0900_0002_0003": "Extrusion abnormal; possible clog or slipping filament",
		"0300_1A00_0002_0002": "Nozzle clogged with filament",
		"07FF_2000_0002_0008": "Auxiliary extruder cannot unload filament; worn filament may be slipping",
		"07FF_2000_0002_0009": "Auxiliary extruder cannot feed filament; check for a blocked tube or slipping filament",
	} {
		messages[code] = text
	}
}

func hmsCode(attr, code uint32) string {
	return fmt.Sprintf("%04X_%04X_%04X_%04X", attr>>16, attr&65535, code>>16, code&65535)
}

func filamentUnit(prefix uint32, unit int) string {
	if prefix == 0x18 {
		return AMSLabel(128 + unit)
	}
	return AMSLabel(unit)
}

var printErrors = buildPrintErrors()

func buildPrintErrors() map[string]string {
	messages := map[string]string{
		"03008013": "Print paused by user",
		"03004006": "Nozzle clogged with filament",
		"03008016": "Nozzle clogged with filament",
		"0300801C": "Extrusion resistance abnormal; possible extruder clog",
		"0300801E": "Extruder motor overloaded; possible clog or filament jam",
	}
	for _, prefix := range []uint32{0x07, 0x18} {
		for unit := range 8 {
			for code, text := range map[uint32]string{
				0x8003: "filament cannot leave the extruder; possible clog or broken filament",
				0x8004: "filament retraction failed; check the spool and filament path",
				0x8005: "filament feeding failed; check the filament end and spool movement",
				0x8006: "Unable to feed filament into the extruder; check the spool and PTFE path",
				0x8007: "extrusion failed; check for an extruder clog",
				0x8010: "assist motor overloaded; filament may be tangled or the spool stuck",
				0x8011: "filament runout; replace filament in the same AMS slot",
				0x8013: "purge timed out; filament may be stuck or the extruder clogged",
			} {
				key := fmt.Sprintf("%02X%02X%04X", prefix, unit, code)
				messages[key] = filamentUnit(prefix, unit) + ": " + text
			}
		}
		for _, port := range []uint32{0xFE, 0xFF} {
			for code, text := range map[uint32]string{
				0x8004: "External spool retraction failed; filament may be stuck in the extruder",
				0x8006: "External spool filament needs loading into the extruder",
				0x8011: "External spool filament runout; load a replacement spool",
				0x8013: "External spool purge timed out; check for stuck filament or an extruder clog",
			} {
				messages[fmt.Sprintf("%02X%02X%04X", prefix, port, code)] = text
			}
		}
	}
	return messages
}
