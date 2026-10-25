package farm

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

type (
	// Attention identifies an unreviewed condition that warrants operator attention.
	// Key and Since preserve the underlying notice's independent occurrence identity.
	Attention struct {
		Key, PrinterID, Printer, Text, Code, Guidance string
		Since                                         time.Time
		// Categories includes contributing fault categories for a related pause-state notice.
		Categories []string
	}

	faultAction struct {
		guidance                     string
		known, recovering, userPause bool
	}

	// printerAttention collects verified reasons that affect a separate pause-state alert.
	printerAttention struct {
		intervention, recovering, userPause bool
		categories                          []string
	}
)

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
		stopped := p.State == StatePaused || p.State == StateFailed
		context := printerAttention{}
		add := func(key, guidance string, categories ...string) bool {
			c, ok := s.Active[key]
			if !ok || c.Acknowledged || c.Dismissed {
				return false
			}
			items = append(items, Attention{Key: key, PrinterID: id, Printer: p.Name, Text: c.Text, Code: c.Code, Guidance: guidance, Since: c.Since, Categories: categories})
			return true
		}
		for _, f := range p.Faults {
			action := classifyFaultAction(f)
			context.recovering = context.recovering || action.recovering
			context.userPause = context.userPause || action.userPause
			guidance := action.guidance
			if !action.known && stopped && f.Level != LevelInfo {
				guidance = reviewPrinter // Preserve an unknown fault without guessing its cause.
			}
			if guidance != "" {
				context.intervention = true
				category := faultAlertCategory(guidance)
				if add(id+"/"+ConditionFaultPrefix+f.Code, guidance, category) && !slices.Contains(context.categories, category) {
					context.categories = append(context.categories, category)
				}
			}
		}
		// A reviewed fault must not re-prompt solely because its paused state was
		// reported separately. A new failure remains independently actionable.
		if p.State == StateFailed || context.pauseNeedsReview(p) {
			categories := context.categories
			if p.State == StateFailed {
				categories = []string{AlertFailedPrints}
			} else if len(categories) == 0 {
				categories = []string{AlertUnknownPauses}
			}
			add(id+"/"+ConditionState, reviewPrinter, categories...)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

// classifyFaultAction uses verified code metadata, never message-text matching.
func classifyFaultAction(f Fault) faultAction {
	code := strings.ReplaceAll(f.Code, "_", "")
	if len(code) == printCodeHexDigits {
		if _, known := printErrors[code]; !known {
			return faultAction{}
		}
		if action, ok := printActions[code]; ok {
			action.known = true
			return action
		}
		raw, _ := strconv.ParseUint(code, 16, 32)
		prefix := raw >> faultPrefixShift
		if prefix == amsFaultPrefix || prefix == amsHTFaultPrefix {
			return faultAction{known: true, guidance: printFilamentGuidance[uint32(raw&faultWordMask)]}
		}
		return faultAction{known: true}
	}
	if _, known := hmsMessages[f.Code]; !known {
		return faultAction{}
	}
	raw, _ := strconv.ParseUint(code, 16, 64)
	prefix, part, detail := raw>>hmsPrefixShift, uint16(raw>>hmsPartShift), uint32(raw)
	if prefix == amsFaultPrefix || prefix == amsHTFaultPrefix {
		if part >= hmsSlotComponent && part <= hmsLastSlotComponent && (detail == hmsAutoPurge || detail == hmsBackupSwitch) {
			return faultAction{known: true, recovering: true}
		}
		component := part & hmsComponentMask
		for _, rule := range hmsActionRules {
			candidate := component
			if rule.exactPart {
				candidate = part
			}
			if candidate == rule.component && slices.Contains(rule.details, detail) {
				return faultAction{known: true, guidance: rule.guidance}
			}
		}
	}
	return faultAction{known: true, guidance: hmsExactGuidance[f.Code]}
}

// faultAlertCategory derives categories from verified action metadata, never printer text.
func faultAlertCategory(guidance string) string {
	switch guidance {
	case replaceFilament, checkFilamentPath:
		return AlertFilament
	case checkExtrusion:
		return AlertExtrusion
	default:
		return AlertPrinterFaults
	}
}

// pauseNeedsReview waits for complete snapshots and excludes explained pauses.
func (a printerAttention) pauseNeedsReview(p *Printer) bool {
	if p.State != StatePaused {
		return false
	}
	if len(a.categories) > 0 {
		return true
	}
	if a.intervention || a.recovering || a.userPause {
		return false
	}
	_, hmsReported := p.Raw[fieldHMS]
	_, printErrorReported := p.Raw[fieldPrintError]
	return hmsReported && printErrorReported
}
