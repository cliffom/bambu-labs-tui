// Package farm owns the normalized farm state. Only the UI reducer
// mutates it; transports deliver immutable reports through messages.
package farm

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Device struct {
	ID        string `json:"dev_id"`
	Name      string `json:"name"`
	Model     string `json:"dev_product_name"`
	ModelCode string `json:"dev_model_name"`
	Online    bool   `json:"online"`
}

type Temperature struct{ Current, Target *int }
type Nozzle struct {
	ID          int
	Temperature Temperature
	Active      bool
	AMS, Slot   *int
}
type Slot struct {
	ID              int
	Material, Color string
	Present         *bool
	Remaining       *int
	Nozzles         []int
}
type AMS struct {
	ID          int
	Humidity    *int
	Temperature *int
	Slots       []Slot
}
type Fault struct{ Code, Text, Level string }
type Printer struct {
	Device
	Firmware                                Firmware
	Raw                                     map[string]any
	LastReport                              time.Time
	RemainingReported                       time.Time
	State, Job, Stage                       string
	Progress, Remaining, Layer, TotalLayers *int
	Bed, Chamber                            Temperature
	Nozzles                                 []Nozzle
	AMS                                     []AMS
	External                                []Slot
	Faults                                  []Fault
}
type Event struct {
	ID                   uint64
	At                   time.Time
	Printer, Level, Text string
	Code                 string
	ConditionKey         string
}
type Condition struct {
	Printer, Level, Text string
	Code                 string
	Since                time.Time
	Acknowledged         bool
	Dismissed            bool
}
type State struct {
	Printers                  map[string]*Printer
	Order                     []string
	Events                    []Event
	Active                    map[string]Condition
	Connected                 bool
	CloudProblem              string
	LowFilament               int
	nextEventID               uint64
	reviews                   map[string]Review
	reviewRevision            uint64
	discovered, cloudObserved bool
}

func New(threshold int) *State {
	return &State{Printers: map[string]*Printer{}, Active: map[string]Condition{}, LowFilament: threshold}
}

func (s *State) Discover(devices []Device, now time.Time) {
	s.discovered = true
	seen := map[string]bool{}
	for _, d := range devices {
		if d.ID == "" {
			continue
		}
		seen[d.ID] = true
		if d.Name == "" {
			d.Name = d.Model
		}
		if d.Name == "" {
			d.Name = "Bambu printer"
		}
		p := s.Printers[d.ID]
		if p == nil {
			p = &Printer{Raw: map[string]any{}, State: "UNKNOWN"}
			s.Printers[d.ID] = p
		}
		p.Device = d
	}
	for id := range s.Printers {
		if !seen[id] {
			delete(s.Printers, id)
		}
	}
	s.Order = s.Order[:0]
	for id := range s.Printers {
		s.Order = append(s.Order, id)
	}
	sort.Slice(s.Order, func(i, j int) bool {
		a, b := s.Printers[s.Order[i]], s.Printers[s.Order[j]]
		if a.Name == b.Name {
			return a.ID < b.ID
		}
		return a.Name < b.Name
	})
	s.Reconcile(now)
}

func (s *State) Cloud(connected bool, problem string, now time.Time) {
	s.cloudObserved = true
	s.Connected, s.CloudProblem = connected, problem
	s.Reconcile(now)
}

func (s *State) Report(id string, payload []byte, now time.Time) error {
	p := s.Printers[id]
	if p == nil {
		return nil
	}
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("invalid status JSON")
	}
	p.versionReport(object(envelope["info"]), now)
	delta, ok := envelope["print"].(map[string]any)
	if !ok {
		return nil
	}
	if cmd := str(delta["command"]); cmd != "" && cmd != "push_status" {
		return nil
	}
	previous := p.State
	// A new job must not inherit the previous job's ETA/progress while its
	// initial report is still arriving.
	task := str(delta["task_id"])
	newTask := task != "" && str(p.Raw["task_id"]) != "" && task != str(p.Raw["task_id"])
	restart := str(delta["gcode_state"]) == "RUNNING" && (previous == "FINISH" || previous == "FAILED" || previous == "IDLE")
	if newTask || restart {
		for _, k := range []string{"mc_remaining_time", "mc_percent", "layer_num", "total_layer_num", "subtask_name", "gcode_file"} {
			delete(p.Raw, k)
		}
		p.RemainingReported = time.Time{}
	}
	if _, ok := delta["mc_remaining_time"]; ok {
		p.RemainingReported = now
	}
	merge(p.Raw, delta)
	p.firmwareReport(delta, now)
	p.LastReport, p.Online = now, true
	p.normalize()
	if previous != p.State && previous != "UNKNOWN" {
		level := "info"
		if p.State == "FAILED" || p.State == "PAUSE" {
			level = "warning"
		}
		s.add(Event{At: now, Printer: p.Name, Level: level, Text: "Print state: " + label(previous) + " → " + label(p.State)})
	}
	s.Reconcile(now)
	return nil
}

func (p *Printer) Stale(now time.Time) bool {
	return p.LastReport.IsZero() || now.Sub(p.LastReport) > 120*time.Second
}
func (p *Printer) Connection(now time.Time, cloud bool) string {
	if !cloud {
		return "Cloud disconnected"
	}
	if !p.Online {
		return "Offline"
	}
	if p.LastReport.IsZero() {
		return "Waiting for data"
	}
	if p.Stale(now) {
		return "Stale"
	}
	return "Live"
}
func (p *Printer) ETA() (time.Time, bool) {
	if p.State != "RUNNING" || p.Remaining == nil || p.RemainingReported.IsZero() {
		return time.Time{}, false
	}
	return p.RemainingReported.Add(time.Duration(*p.Remaining) * time.Minute), true
}

func (s *State) add(e Event) {
	s.nextEventID++
	e.ID = s.nextEventID
	s.Events = append(s.Events, e)
	if len(s.Events) > 500 {
		s.Events = append([]Event(nil), s.Events[len(s.Events)-500:]...)
	}
}

// Acknowledge marks an ongoing condition as reviewed locally. It never
// edits printer telemetry or sends a printer command. Resolution and recurrence
// create a new, unacknowledged condition.
func (s *State) Acknowledge(key string, acknowledged bool) bool {
	c, ok := s.Active[key]
	if !ok {
		return false
	}
	c.Acknowledged = acknowledged
	s.Active[key] = c
	s.rememberReview(key, c)
	return true
}

// Dismiss hides this occurrence locally, including its matching history entry.
// The underlying telemetry stays intact so recurrence can be detected.
func (s *State) Dismiss(key string) bool {
	c, ok := s.Active[key]
	if !ok {
		return false
	}
	c.Dismissed = true
	s.Active[key] = c
	s.rememberReview(key, c)
	events := s.Events[:0]
	for _, e := range s.Events {
		if e.ConditionKey != key || e.At.Before(c.Since) {
			events = append(events, e)
		}
	}
	s.Events = events
	return true
}

func (s *State) NoticeCount() int {
	n := 0
	for _, c := range s.Active {
		if !c.Dismissed {
			n++
		}
	}
	return n
}

func (s *State) DeleteEvent(id uint64) bool {
	for i, e := range s.Events {
		if e.ID == id {
			s.Events = append(s.Events[:i], s.Events[i+1:]...)
			return true
		}
	}
	return false
}
func (s *State) ClearEvents() { s.Events = nil }
func (s *State) Unacknowledged() int {
	n := 0
	for _, c := range s.Active {
		if !c.Acknowledged && !c.Dismissed {
			n++
		}
	}
	return n
}
func (s *State) Reconcile(now time.Time) {
	next := map[string]Condition{}
	if !s.Connected {
		text := "Cloud disconnected; reconnecting"
		if s.CloudProblem != "" {
			text = s.CloudProblem
		}
		next["cloud"] = Condition{Printer: "Farm", Level: "warning", Text: text}
	}
	for id, p := range s.Printers {
		if !p.Online {
			next[id+"/offline"] = Condition{Printer: p.Name, Level: "warning", Text: "Printer offline"}
		} else if s.Connected && !p.LastReport.IsZero() && p.Stale(now) {
			next[id+"/stale"] = Condition{Printer: p.Name, Level: "warning", Text: "Telemetry stale; last report " + p.LastReport.Local().Format("15:04:05")}
		}
		for _, f := range p.Faults {
			level := f.Level
			if level == "" {
				level = "warning"
			}
			next[id+"/fault/"+f.Code] = Condition{Printer: p.Name, Level: level, Text: f.Text + " [" + f.Code + "]", Code: f.Code}
		}
		for _, a := range p.AMS {
			for _, slot := range a.Slots {
				if slot.Present != nil && !*slot.Present {
					continue
				}
				if slot.Remaining != nil && *slot.Remaining <= s.LowFilament {
					key := fmt.Sprintf("%s/low/%d/%d", id, a.ID, slot.ID)
					next[key] = Condition{Printer: p.Name, Level: "warning", Text: fmt.Sprintf("%s slot %d: estimated filament %d%%", AMSLabel(a.ID), slot.ID+1, *slot.Remaining)}
				}
			}
		}
		if p.State == "PAUSE" || p.State == "FAILED" {
			next[id+"/state"] = Condition{Printer: p.Name, Level: "warning", Text: "Print " + label(p.State)}
		}
	}
	keys := make([]string, 0, len(next))
	for k := range next {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Restored reviews must survive discovery, ticks, and partial reports until
	// telemetry has actually supplied the relevant condition family.
	for k := range s.reviews {
		if _, active := next[k]; !active && s.reviewKnown(k) {
			s.forgetReview(k)
		}
	}
	for _, k := range keys {
		c := next[k]
		old, ongoing := s.Active[k]
		// An expired session or failed print is a new condition even if a
		// previous connection problem or pause used the same list identity.
		changed := old.Level != c.Level || old.Code != c.Code || ((k == "cloud" || strings.HasSuffix(k, "/state")) && old.Text != c.Text)
		if ongoing && !changed {
			c.Since, c.Acknowledged = old.Since, old.Acknowledged
			c.Dismissed = old.Dismissed
		} else {
			c.Since = now
			if review, ok := s.reviews[k]; ok {
				if review.matches(k, c) {
					c.Since, c.Acknowledged, c.Dismissed = review.Since, review.Acknowledged, review.Dismissed
				} else {
					s.forgetReview(k)
				}
			}
			if !c.Dismissed {
				s.add(Event{At: now, Printer: c.Printer, Level: c.Level, Text: c.Text, Code: c.Code, ConditionKey: k})
			}
		}
		next[k] = c
	}
	keys = keys[:0]
	for k := range s.Active {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := next[k]; !ok {
			c := s.Active[k]
			if !c.Dismissed {
				s.add(Event{At: now, Printer: c.Printer, Level: "info", Text: "Resolved: " + c.Text, Code: c.Code, ConditionKey: k})
			}
		}
	}
	s.Active = next
}

func label(s string) string {
	switch s {
	case "RUNNING":
		return "printing"
	case "PAUSE":
		return "paused"
	case "FINISH":
		return "finished"
	case "FAILED":
		return "failed"
	default:
		return strings.ToLower(s)
	}
}

// merge preserves omitted properties and identity-bearing arrays; explicit empty
// arrays clear a collection. Fault lists are snapshots rather than deltas.
func merge(dst, src map[string]any) {
	for k, v := range src {
		if obj, ok := v.(map[string]any); ok {
			old, _ := dst[k].(map[string]any)
			if old == nil {
				old = map[string]any{}
			}
			merge(old, obj)
			dst[k] = old
			continue
		}
		if a, ok := v.([]any); ok && len(a) > 0 && (k == "ams" || k == "tray" || k == "info" || k == "vir_slot") {
			old, _ := dst[k].([]any)
			for _, item := range a {
				obj, ok := item.(map[string]any)
				if !ok {
					continue
				}
				id, hasID := obj["id"]
				if !hasID {
					continue
				}
				// Legacy firmware reports an empty tray as just its identity.
				if k == "tray" && len(obj) == 1 {
					obj["state"] = float64(0)
				}
				found := false
				for _, existing := range old {
					m, ok := existing.(map[string]any)
					if ok && str(m["id"]) == str(id) {
						if k == "tray" {
							if state := number(obj["state"]); state != nil && *state&1 == 0 {
								for key := range m {
									if key != "id" {
										delete(m, key)
									}
								}
							}
						}
						// A replacement spool must not inherit the previous spool's estimate.
						for _, identity := range []string{"tray_uuid", "tag_uid"} {
							if v := str(obj[identity]); v != "" && v != str(m[identity]) {
								delete(m, "remain")
							}
						}
						merge(m, obj)
						found = true
						break
					}
				}
				if !found {
					m := map[string]any{}
					merge(m, obj)
					old = append(old, m)
				}
			}
			dst[k] = old
			continue
		}
		dst[k] = v
	}
}

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func number(v any) *int {
	var n float64
	var err error
	switch x := v.(type) {
	case float64:
		n = x
	case string:
		n, err = strconv.ParseFloat(x, 64)
	default:
		return nil
	}
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n > math.MaxUint32 || n < math.MinInt32 {
		return nil
	}
	i := int(n)
	return &i
}
func bounded(v any, min, max int) *int {
	n := number(v)
	if n == nil || *n < min || *n > max {
		return nil
	}
	return n
}
func ptr(n int) *int       { return &n }
func boolean(b bool) *bool { return &b }
func packed(v any) Temperature {
	n := number(v)
	if n == nil {
		return Temperature{}
	}
	return Temperature{ptr(*n & 0xffff), ptr((*n >> 16) & 0xffff)}
}

func (p *Printer) normalize() {
	r := p.Raw
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
