package farm

import (
	"fmt"
	"sort"
)

func (p *Printer) normalize() {
	r := p.Raw
	p.knowledge = decodeConditionKnowledge(r)
	p.State = str(r[fieldGcodeState])
	if p.State == "" {
		p.State = StateUnknown
	}
	p.Job = str(r[fieldSubtaskName])
	if p.Job == "" {
		p.Job = str(r[fieldGcodeFile])
	}
	p.Progress = bounded(r[fieldProgress], 0, MaxPercent)
	p.Remaining = bounded(r[fieldRemainingMinutes], 0, MaxRemainingMinutes)
	p.Layer = bounded(r[fieldLayerNum], 0, MaxLayers)
	p.TotalLayers = bounded(r[fieldTotalLayerNum], 0, MaxLayers)
	p.Stage = "Unknown"
	// The print stage can linger after a job ends (including code 0,
	// "Printing"). The overall job state takes precedence in terminal states.
	switch p.State {
	case StateIdle:
		p.Stage = "Idle"
	case StateFinished:
		p.Stage = "Finished"
	case StateFailed:
		p.Stage = "Failed"
	default:
		if n := number(r[fieldStage]); n != nil && *n >= 0 {
			p.Stage = stage(*n)
		}
	}
	p.Bed = Temperature{number(r[fieldBedTemper]), number(r[fieldBedTargetTemper])}
	p.Chamber = Temperature{number(r[fieldChamberTemper]), number(r[fieldChamberTargetTemper])}
	d := object(r[fieldDevice])
	if v := object(object(d[fieldBed])[fieldInfo])[fieldTemp]; v != nil {
		p.Bed = packed(v)
	} else if v := d[fieldBedTemp]; v != nil {
		p.Bed = packed(v)
	}
	if v := object(object(d[fieldCtc])[fieldInfo])[fieldTemp]; v != nil {
		p.Chamber = packed(v)
	}
	ex := object(d[fieldExtruder])
	active := -1
	if n := number(ex[fieldState]); n != nil {
		active = (*n >> activeNozzleShift) & nozzleIndexMask
	}
	p.Nozzles = nil
	for _, item := range array(ex[fieldInfo]) {
		m := object(item)
		id := number(m[fieldID])
		if id == nil || *id < 0 || *id > LeftNozzle {
			continue
		}
		n := Nozzle{ID: *id, Temperature: packed(m[fieldTemp]), Active: *id == active}
		if snow := number(m[fieldSnow]); snow != nil {
			n.AMS = ptr(*snow >> unitIDShift)
			n.Slot = ptr(*snow & slotIDMask)
			if *n.Slot >= SlotsPerAMS {
				n.AMS = nil
				n.Slot = nil
			}
		}
		p.Nozzles = append(p.Nozzles, n)
	}
	if len(p.Nozzles) == 0 {
		n := Nozzle{ID: 0, Temperature: Temperature{number(r[fieldNozzleTemper]), number(r[fieldNozzleTargetTemper])}, Active: true}
		if t := number(object(r[fieldAMS])[fieldTrayNow]); t != nil {
			switch {
			case *t == legacyExternalTray:
				n.AMS = ptr(LegacyExternalSpoolID)
				n.Slot = ptr(0)
			case *t == legacyNoTray:
			case *t >= AMSHTFirstID:
				n.AMS = ptr(*t)
				n.Slot = ptr(0)
			default:
				n.AMS = ptr(*t >> legacySlotBits)
				n.Slot = ptr(*t & legacySlotMask)
			}
		}
		p.Nozzles = []Nozzle{n}
	}
	sort.Slice(p.Nozzles, func(i, j int) bool { return p.Nozzles[i].ID < p.Nozzles[j].ID })
	p.AMS = nil
	for _, item := range array(object(r[fieldAMS])[fieldAMS]) {
		m := object(item)
		id := number(m[fieldID])
		if id == nil {
			continue
		}
		a := AMS{ID: *id, Humidity: bounded(m[fieldHumidityRaw], 0, MaxPercent), Temperature: bounded(m[fieldTemp], 0, maxAMSTemperature)}
		for _, item := range array(m[fieldTray]) {
			v := object(item)
			slot := decodeSlot(v)
			if number(v[fieldID]) == nil {
				continue
			}
			for _, n := range p.Nozzles {
				if n.AMS != nil && n.Slot != nil && *n.AMS == a.ID && *n.Slot == slot.ID {
					slot.Nozzles = append(slot.Nozzles, n.ID)
				}
			}
			a.Slots = append(a.Slots, slot)
		}
		sort.Slice(a.Slots, func(i, j int) bool { return a.Slots[i].ID < a.Slots[j].ID })
		p.AMS = append(p.AMS, a)
	}
	sort.Slice(p.AMS, func(i, j int) bool { return p.AMS[i].ID < p.AMS[j].ID })
	p.External = nil
	virtual := array(r[fieldVirSlot])
	legacyExternal := false
	if v := object(r[fieldVtTray]); len(virtual) == 0 && v != nil {
		virtual = []any{v}
		legacyExternal = true
	}
	for _, item := range virtual {
		v := object(item)
		id := number(v[fieldID])
		if id == nil {
			continue
		}
		slot := decodeSlot(v)
		if legacyExternal {
			slot.ID = LegacyExternalSpoolID
		}
		slot.Remaining = nil
		for _, n := range p.Nozzles {
			if n.AMS != nil && *n.AMS == slot.ID {
				slot.Nozzles = append(slot.Nozzles, n.ID)
			}
		}
		p.External = append(p.External, slot)
	}
	p.Faults = nil
	if n := number(r[fieldPrintError]); n != nil && *n != 0 {
		code := fmt.Sprintf("%08X", uint32(*n))
		text := "Printer error"
		if t, ok := printErrors[code]; ok {
			text = t
		}
		p.Faults = append(p.Faults, Fault{Code: code[:faultWordHexDigits] + "_" + code[faultWordHexDigits:], Text: text, Level: LevelError})
	}
	for _, item := range array(r[fieldHMS]) {
		m := object(item)
		a, c := number(m[fieldAttr]), number(m[fieldCode])
		if a == nil || c == nil {
			continue
		}
		code := hmsCode(uint32(*a), uint32(*c))
		text := str(m[fieldMessage])
		if text == "" {
			var known bool
			text, known = hmsMessages[code]
			if known && text == "" {
				continue
			}
			if !known {
				text = "Unrecognized HMS notice"
			}
		}
		p.Faults = append(p.Faults, Fault{Code: code, Text: text, Level: hmsLevel(*c)})
	}
}

func decodeSlot(m map[string]any) Slot {
	id := number(m[fieldID])
	s := Slot{Material: str(m[fieldTrayType]), Color: str(m[fieldTrayColor]), Remaining: bounded(m[fieldRemain], 0, MaxPercent)}
	if id != nil {
		s.ID = *id
	}
	if n := number(m[fieldState]); n != nil {
		// Presence is physical spool presence. Reading/scanning states can
		// contain a spool without trustworthy metadata or remaining quantity.
		s.Present = boolean(*n&spoolPresentMask != 0)
		if *n != spoolReadyState && *n&spoolMetadataMask == 0 {
			s.Material = ""
			s.Color = ""
			s.Remaining = nil
		}
	} else if s.Material != "" {
		s.Present = boolean(s.Material != emptyMaterial)
	}
	if s.Present != nil && !*s.Present {
		s.Material = ""
		s.Color = ""
		s.Remaining = nil
	}
	return s
}

func stage(n int) string {
	stages := map[int]string{0: "Printing", 1: "Bed leveling", 2: "Preheating bed", 4: "Changing filament", 5: "Paused by G-code", 6: "Filament runout", 7: "Heating nozzle", 8: "Calibrating extrusion", 9: "Scanning bed", 10: "Inspecting first layer", 13: "Homing", 14: "Cleaning nozzle", 16: "Paused by user", 22: "Unloading filament", 24: "Loading filament", 29: "Cooling chamber", 35: "Nozzle clog", 49: "Heating chamber", 255: "Idle"}
	if s, ok := stages[n]; ok {
		return s
	}
	return fmt.Sprintf("Stage %d", n)
}
