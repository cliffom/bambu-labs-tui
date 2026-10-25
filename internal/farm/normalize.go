package farm

import (
	"fmt"
	"sort"
)

func (p *Printer) normalize() {
	r := p.Raw
	p.knowledge = decodeConditionKnowledge(r)
	p.State = str(r["gcode_state"])
	if p.State == "" {
		p.State = "UNKNOWN"
	}
	p.Job = str(r["subtask_name"])
	if p.Job == "" {
		p.Job = str(r["gcode_file"])
	}
	p.Progress = bounded(r["mc_percent"], 0, 100)
	p.Remaining = bounded(r["mc_remaining_time"], 0, 525600)
	p.Layer = bounded(r["layer_num"], 0, 1000000)
	p.TotalLayers = bounded(r["total_layer_num"], 0, 1000000)
	p.Stage = "Unknown"
	if n := number(r["stg_cur"]); n != nil {
		p.Stage = stage(*n)
	}
	p.Bed = Temperature{number(r["bed_temper"]), number(r["bed_target_temper"])}
	p.Chamber = Temperature{number(r["chamber_temper"]), number(r["chamber_target_temper"])}
	d := object(r["device"])
	if v := object(object(d["bed"])["info"])["temp"]; v != nil {
		p.Bed = packed(v)
	} else if v := d["bed_temp"]; v != nil {
		p.Bed = packed(v)
	}
	if v := object(object(d["ctc"])["info"])["temp"]; v != nil {
		p.Chamber = packed(v)
	}
	ex := object(d["extruder"])
	active := -1
	if n := number(ex["state"]); n != nil {
		active = (*n >> 4) & 15
	}
	p.Nozzles = nil
	for _, item := range array(ex["info"]) {
		m := object(item)
		id := number(m["id"])
		if id == nil || *id < 0 || *id > 1 {
			continue
		}
		n := Nozzle{ID: *id, Temperature: packed(m["temp"]), Active: *id == active}
		if snow := number(m["snow"]); snow != nil {
			n.AMS = ptr(*snow >> 8)
			n.Slot = ptr(*snow & 15)
			if *n.Slot > 3 {
				n.AMS = nil
				n.Slot = nil
			}
		}
		p.Nozzles = append(p.Nozzles, n)
	}
	if len(p.Nozzles) == 0 {
		n := Nozzle{ID: 0, Temperature: Temperature{number(r["nozzle_temper"]), number(r["nozzle_target_temper"])}, Active: true}
		if t := number(object(r["ams"])["tray_now"]); t != nil {
			switch {
			case *t == 254:
				n.AMS = ptr(255)
				n.Slot = ptr(0)
			case *t == 255:
			case *t >= 128:
				n.AMS = ptr(*t)
				n.Slot = ptr(0)
			default:
				n.AMS = ptr(*t >> 2)
				n.Slot = ptr(*t & 3)
			}
		}
		p.Nozzles = []Nozzle{n}
	}
	sort.Slice(p.Nozzles, func(i, j int) bool { return p.Nozzles[i].ID < p.Nozzles[j].ID })
	p.AMS = nil
	for _, item := range array(object(r["ams"])["ams"]) {
		m := object(item)
		id := number(m["id"])
		if id == nil {
			continue
		}
		a := AMS{ID: *id, Humidity: bounded(m["humidity_raw"], 0, 100), Temperature: bounded(m["temp"], 0, 100)}
		for _, item := range array(m["tray"]) {
			v := object(item)
			slot := decodeSlot(v)
			if number(v["id"]) == nil {
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
	virtual := array(r["vir_slot"])
	legacyExternal := false
	if v := object(r["vt_tray"]); len(virtual) == 0 && v != nil {
		virtual = []any{v}
		legacyExternal = true
	}
	for _, item := range virtual {
		v := object(item)
		id := number(v["id"])
		if id == nil {
			continue
		}
		slot := decodeSlot(v)
		if legacyExternal {
			slot.ID = 255
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
	if n := number(r["print_error"]); n != nil && *n != 0 {
		code := fmt.Sprintf("%08X", uint32(*n))
		text := "Printer error"
		if t, ok := printErrors[code]; ok {
			text = t
		}
		p.Faults = append(p.Faults, Fault{Code: code[:4] + "_" + code[4:], Text: text, Level: "error"})
	}
	for _, item := range array(r["hms"]) {
		m := object(item)
		a, c := number(m["attr"]), number(m["code"])
		if a == nil || c == nil {
			continue
		}
		code := hmsCode(uint32(*a), uint32(*c))
		text := str(m["message"])
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
	id := number(m["id"])
	s := Slot{Material: str(m["tray_type"]), Color: str(m["tray_color"]), Remaining: bounded(m["remain"], 0, 100)}
	if id != nil {
		s.ID = *id
	}
	if n := number(m["state"]); n != nil {
		// Presence is physical spool presence. Reading/scanning states can
		// contain a spool without trustworthy metadata or remaining quantity.
		s.Present = boolean(*n&1 != 0)
		if *n != 3 && *n&8 == 0 {
			s.Material = ""
			s.Color = ""
			s.Remaining = nil
		}
	} else if s.Material != "" {
		s.Present = boolean(s.Material != "Empty")
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
