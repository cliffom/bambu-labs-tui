// Package farm owns the normalized farm state. Only the UI reducer
// mutates it; transports deliver immutable reports through messages.
package farm

import (
	"sort"
	"time"
)

type (
	Device struct {
		ID        string `json:"dev_id"`
		Name      string `json:"name"`
		Model     string `json:"dev_product_name"`
		ModelCode string `json:"dev_model_name"`
		Online    bool   `json:"online"`
	}

	Temperature struct{ Current, Target *int }

	Nozzle struct {
		ID          int
		Temperature Temperature
		Active      bool
		AMS, Slot   *int
	}

	Slot struct {
		ID              int
		Material, Color string
		Present         *bool
		Remaining       *int
		Nozzles         []int
	}

	AMS struct {
		ID          int
		Humidity    *int
		Temperature *int
		Slots       []Slot
	}

	Fault struct{ Code, Text, Level string }

	Printer struct {
		Device
		Firmware Firmware
		// Raw is the flexible merged snapshot for telemetry interpretation and
		// allowlisted diagnostics; lifecycle decisions use normalized fields.
		Raw                                     map[string]any
		knowledge                               conditionKnowledge
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

	Event struct {
		ID                   uint64
		At                   time.Time
		Printer, Level, Text string
		Code                 string
		ConditionKey         string
	}

	Condition struct {
		Printer, Level, Text string
		Code                 string
		Since                time.Time
		Acknowledged         bool
		Dismissed            bool
	}

	State struct {
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
)

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
			p = &Printer{Raw: map[string]any{}, State: StateUnknown}
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

// Report applies interpreted telemetry, then records lifecycle transitions and
// reconciles conditions. Protocol fields are owned by the telemetry layer.
func (s *State) Report(id string, payload []byte, now time.Time) error {
	p := s.Printers[id]
	if p == nil {
		return nil
	}
	previous := p.State
	applied, err := p.applyReport(payload, now)
	if err != nil || !applied {
		return err
	}
	if previous != p.State && previous != StateUnknown {
		level := LevelInfo
		if p.State == StateFailed || p.State == StatePaused {
			level = LevelWarning
		}
		s.add(Event{At: now, Printer: p.Name, Level: level, Text: "Print state: " + label(previous) + " → " + label(p.State)})
	}
	s.Reconcile(now)
	return nil
}

func (p *Printer) Stale(now time.Time) bool {
	threshold := activeTelemetryMaxAge
	if p.State == StateIdle || p.State == StateFinished {
		threshold = idleTelemetryMaxAge
	}
	return p.LastReport.IsZero() || now.Sub(p.LastReport) > threshold
}

func (p *Printer) Connection(now time.Time, cloud bool) string {
	if !cloud {
		return ConnectionCloudDisconnected
	}
	if !p.Online {
		return ConnectionOffline
	}
	if p.LastReport.IsZero() {
		return ConnectionWaiting
	}
	if p.Stale(now) {
		return ConnectionStale
	}
	return ConnectionLive
}

func (p *Printer) ETA() (time.Time, bool) {
	if p.State != StateRunning || p.Remaining == nil || p.RemainingReported.IsZero() {
		return time.Time{}, false
	}
	return p.RemainingReported.Add(time.Duration(*p.Remaining) * time.Minute), true
}

func (s *State) add(e Event) {
	s.nextEventID++
	e.ID = s.nextEventID
	s.Events = append(s.Events, e)
	if len(s.Events) > MaxEventHistory {
		s.Events = append([]Event(nil), s.Events[len(s.Events)-MaxEventHistory:]...)
	}
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
