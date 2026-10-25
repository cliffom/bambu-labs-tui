package farm

type (
	// conditionKnowledge records whether telemetry has supplied enough information
	// to reconcile an absent condition. Observation differs from a valid measurement:
	// an explicit unknown estimate still supersedes an older estimate, while an
	// omitted estimate must preserve a restored review until its slot is known.
	// These fields are internal interpretation metadata, not diagnostic export data.
	conditionKnowledge struct {
		printState, printerFaults, hmsFaults bool
		inventoryEmpty                       bool
		units                                []unitKnowledge
	}

	unitKnowledge struct {
		id    int
		empty bool
		slots map[int]bool
	}
)

func (k conditionKnowledge) lowFilamentKnown(unitID, slotID int) bool {
	if k.inventoryEmpty {
		return true
	}
	for _, unit := range k.units {
		if unit.id != unitID {
			continue
		}
		if unit.empty {
			return true
		}
		if known, present := unit.slots[slotID]; present {
			return known
		}
	}
	return false
}

// decodeConditionKnowledge uses the merged snapshot, so omitted deltas retain
// knowledge and explicit empty collections replace it. Identity-bearing arrays
// remain partial: another unit/slot's report establishes nothing about this one.
func decodeConditionKnowledge(r map[string]any) conditionKnowledge {
	var k conditionKnowledge
	_, k.printState = r[fieldGcodeState]
	_, k.printerFaults = r[fieldPrintError]
	_, k.hmsFaults = r[fieldHMS]
	units, reported := object(r[fieldAMS])[fieldAMS].([]any)
	k.inventoryEmpty = reported && len(units) == 0
	for _, entry := range units {
		raw := object(entry)
		id := number(raw[fieldID])
		if id == nil {
			continue
		}
		trays, reported := raw[fieldTray].([]any)
		unit := unitKnowledge{id: *id, empty: reported && len(trays) == 0, slots: map[int]bool{}}
		for _, entry := range trays {
			raw := object(entry)
			slotID := number(raw[fieldID])
			if slotID == nil {
				continue
			}
			if _, seen := unit.slots[*slotID]; seen {
				continue
			}
			_, estimateReported := raw[fieldRemain]
			state := number(raw[fieldState])
			unit.slots[*slotID] = estimateReported || (state != nil && *state&spoolPresentMask == 0) || str(raw[fieldTrayType]) == emptyMaterial
		}
		k.units = append(k.units, unit)
	}
	return k
}
