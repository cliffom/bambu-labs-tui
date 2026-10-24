package farm

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// Attention identifies an unreviewed condition that warrants operator attention.
// Key and Since preserve the underlying notice's independent occurrence identity.
type Attention struct {
	Key, PrinterID, Printer, Text, Code, Guidance string
	Since                                         time.Time
}

type faultAction struct {
	guidance                     string
	known, recovering, userPause bool
}

const (
	reviewPrinter     = "Inspect the printer and its reported fault; use the printer or Bambu app to resolve it."
	replaceFilament   = "Load replacement filament into the indicated path using the printer or Bambu app."
	checkFilamentPath = "Inspect the spool and filament path; follow the printer's troubleshooting instructions."
	checkExtrusion    = "Inspect the extruder/nozzle; follow the printer's troubleshooting instructions."
)

// Attention returns actionable, unacknowledged, undismissed occurrences. Known
// recovery and manual-pause messages do not independently interrupt the operator.
func (s *State) Attention() []Attention {
	var items []Attention
	for id, p := range s.Printers {
		stopped := p.State == "PAUSE" || p.State == "FAILED"
		intervention, recovering, userPause := false, false, false
		faultItems := 0
		add := func(key, guidance string) {
			c, ok := s.Active[key]
			if !ok || c.Acknowledged || c.Dismissed {
				return
			}
			items = append(items, Attention{Key: key, PrinterID: id, Printer: p.Name, Text: c.Text, Code: c.Code, Guidance: guidance, Since: c.Since})
			faultItems++
		}
		for _, f := range p.Faults {
			action := classifyFaultAction(f)
			recovering = recovering || action.recovering
			userPause = userPause || action.userPause
			guidance := action.guidance
			if !action.known && stopped && f.Level != "info" {
				guidance = reviewPrinter // Preserve an unknown fault without guessing its cause.
			}
			if guidance != "" {
				intervention = true
				add(id+"/fault/"+f.Code, guidance)
			}
		}
		// A reviewed fault must not re-prompt solely because its paused state was
		// reported separately. A new failure remains independently actionable.
		_, hmsReported := p.Raw["hms"]
		_, printErrorReported := p.Raw["print_error"]
		if p.State == "FAILED" || (p.State == "PAUSE" && (faultItems > 0 || (hmsReported && printErrorReported && !intervention && !recovering && !userPause))) {
			add(id+"/state", reviewPrinter)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

// classifyFaultAction uses verified code metadata, never message-text matching.
func classifyFaultAction(f Fault) faultAction {
	code := strings.ReplaceAll(f.Code, "_", "")
	if len(code) == 8 {
		if _, known := printErrors[code]; !known {
			return faultAction{}
		}
		if code == "03008013" {
			return faultAction{known: true, userPause: true}
		}
		raw, _ := strconv.ParseUint(code, 16, 32)
		if strings.HasPrefix(code, "03") {
			switch code {
			case "03004006", "03008016", "0300801C", "0300801E":
				return faultAction{known: true, guidance: checkExtrusion}
			}
			return faultAction{known: true}
		}
		if raw>>24 != 0x07 && raw>>24 != 0x18 {
			return faultAction{known: true}
		}
		switch raw & 0xFFFF {
		case 0x8011:
			return faultAction{known: true, guidance: replaceFilament}
		case 0x8007, 0x8013:
			return faultAction{known: true, guidance: checkExtrusion}
		case 0x8003, 0x8004, 0x8005, 0x8006, 0x8010:
			return faultAction{known: true, guidance: checkFilamentPath}
		default:
			return faultAction{known: true}
		}
	}
	if _, known := hmsMessages[f.Code]; !known {
		return faultAction{}
	}
	raw, _ := strconv.ParseUint(code, 16, 64)
	prefix, part, detail := raw>>56, uint16(raw>>32), uint32(raw)
	if prefix == 0x07 || prefix == 0x18 {
		if part >= 0x2000 && part <= 0x2300 && (detail == 0x00030001 || detail == 0x00030002) {
			return faultAction{known: true, recovering: true}
		}
		component := part & 0xFCFF // Remove the two slot-address bits only.
		switch {
		case component == 0x2000 && detail == 0x00020001, part == 0x7000 && detail == 0x00020007:
			return faultAction{known: true, guidance: replaceFilament}
		case component == 0x2000 && detail == 0x00020009, part == 0x7000 && (detail == 0x00020003 || detail == 0x00020006):
			return faultAction{known: true, guidance: checkExtrusion}
		case component == 0x1000 && detail == 0x00020002,
			component == 0x6000 && detail == 0x00020001,
			component == 0x7000 && (detail == 0x00020001 || detail == 0x00020002 || detail == 0x00020004 || detail == 0x00020005),
			component == 0x2000 && (detail == 0x00020002 || detail == 0x00020003 || detail == 0x00020004 || detail == 0x00020005 || detail == 0x00020010 || detail == 0x00020011 || detail == 0x00020012 || detail == 0x00020024),
			f.Code == "07FF_2000_0002_0008" || f.Code == "07FF_2000_0002_0009":
			return faultAction{known: true, guidance: checkFilamentPath}
		default:
			return faultAction{known: true}
		}
	}
	switch f.Code {
	case "0300_0900_0002_0001", "0300_0900_0002_0002", "0300_0900_0002_0003", "0300_1A00_0002_0002":
		return faultAction{known: true, guidance: checkExtrusion}
	case "0300_0100_0001_0007", "0500_0600_0002_0032", "0500_0300_0001_0023", "0500_0300_0001_0025":
		return faultAction{known: true, guidance: reviewPrinter}
	default:
		return faultAction{known: true} // Maintenance and other catalogue entries need no automatic modal.
	}
}
