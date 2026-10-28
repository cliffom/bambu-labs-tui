package session

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
)

type (
	Decisions struct {
		Targets map[string]string
		State   string
	}

	DisplayState struct {
		Decisions string
		At        string
		Demo      bool
		Connected bool
		Summary   DisplaySummary
		Printers  []DisplayPrinter
	}
	DisplaySummary struct {
		Total       int
		Printing    int
		Attention   int
		Unavailable int
	}
	DisplayPrinter struct {
		ID                string
		Name              string
		Model             string
		State             string
		Online            bool
		Connection        string
		Fresh             bool
		LastReport        string
		Job               string
		Stage             string
		Progress          *int
		Remaining         *int
		RemainingReported string
		ETACurrent        bool
		Layer             *int
		TotalLayers       *int
		Bed               DisplayTemperature
		Chamber           DisplayTemperature
		Nozzles           []DisplayNozzle
		AMS               []DisplayAMS
		External          []DisplaySlot
		Conditions        []DisplayCondition
		Attention         bool
	}
	DisplayTemperature struct {
		Current *int
		Target  *int
	}
	DisplayNozzle struct {
		ID          int
		Active      bool
		Temperature DisplayTemperature
	}
	DisplaySlot struct {
		ID        int
		Material  string
		Color     string
		Present   *bool
		Remaining *int
	}
	DisplayAMS struct {
		ID          int
		Label       string
		Humidity    *int
		Temperature *int
		Slots       []DisplaySlot
	}
	DisplayCondition struct {
		ID       string
		Level    string
		Text     string
		Code     string
		Since    string
		Reviewed bool
		Hidden   bool
		Guidance string
	}
)

func DisplayID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func timestamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func temp(t farm.Temperature) DisplayTemperature {
	return DisplayTemperature{Current: copyValue(t.Current), Target: copyValue(t.Target)}
}

func slots(source []farm.Slot) []DisplaySlot {
	result := make([]DisplaySlot, 0, len(source))
	for _, s := range source {
		result = append(result, DisplaySlot{ID: s.ID, Material: s.Material, Color: s.Color, Present: copyValue(s.Present), Remaining: copyValue(s.Remaining)})
	}
	return result
}

// BuildDisplay runs on the serial owner and returns an independent allowlisted value. The DTO
// explicitly excludes Raw, device serials, protocol job IDs, account/session
// information, credentials, storage paths, and backend errors. It contains
// only display values; no mutable farm objects leave the reducer.
func BuildDisplay(state *farm.State, now time.Time, demo bool, decisions ...Decisions) DisplayState {
	result := DisplayState{At: timestamp(now), Demo: demo, Connected: state.Connected, Printers: make([]DisplayPrinter, 0, len(state.Printers))}
	var local Decisions
	if len(decisions) > 0 {
		local = decisions[0]
		result.Decisions = local.State
	}
	guidance := map[string]string{}
	attention := map[string]bool{}
	for _, a := range state.Attention() {
		guidance[a.Key] = a.Guidance
		attention[a.PrinterID] = true
	}
	for id, p := range state.Printers {
		model := p.Model
		if model == "" {
			model = p.ModelCode
		}
		out := DisplayPrinter{ID: DisplayID(id), Name: p.Name, Model: model, State: p.State, Online: p.Online, Connection: p.Connection(now, state.Connected), Fresh: p.TelemetryCurrent(now, state.Connected), LastReport: timestamp(p.LastReport), Job: p.Job, Stage: p.Stage, Progress: copyValue(p.Progress), Remaining: copyValue(p.Remaining), RemainingReported: timestamp(p.RemainingReported), ETACurrent: p.ETACurrent(now, state.Connected), Layer: copyValue(p.Layer), TotalLayers: copyValue(p.TotalLayers), Bed: temp(p.Bed), Chamber: temp(p.Chamber), Nozzles: []DisplayNozzle{}, AMS: []DisplayAMS{}, External: slots(p.External), Conditions: []DisplayCondition{}, Attention: attention[id]}
		for _, n := range p.Nozzles {
			out.Nozzles = append(out.Nozzles, DisplayNozzle{ID: n.ID, Active: n.Active, Temperature: temp(n.Temperature)})
		}
		for _, unit := range p.AMS {
			out.AMS = append(out.AMS, DisplayAMS{ID: unit.ID, Label: farm.AMSLabel(unit.ID), Humidity: copyValue(unit.Humidity), Temperature: copyValue(unit.Temperature), Slots: slots(unit.Slots)})
		}
		for key, c := range state.Active {
			owner, _, _ := strings.Cut(key, "/")
			if owner != id {
				continue
			}
			identity := DisplayID(key + timestamp(c.Since))
			if local.Targets != nil {
				identity = local.Targets[key]
			}
			out.Conditions = append(out.Conditions, DisplayCondition{ID: identity, Level: c.Level, Text: c.Text, Code: c.Code, Since: timestamp(c.Since), Reviewed: c.Acknowledged || c.Dismissed, Hidden: c.Dismissed, Guidance: guidance[key]})
		}
		sort.Slice(out.Conditions, func(i, j int) bool { return out.Conditions[i].ID < out.Conditions[j].ID })
		result.Printers = append(result.Printers, out)
		result.Summary.Total++
		if p.State == farm.StateRunning {
			result.Summary.Printing++
		}
		if out.Attention {
			result.Summary.Attention++
		}
		if !out.Fresh {
			result.Summary.Unavailable++
		}
	}
	sort.Slice(result.Printers, func(i, j int) bool {
		a, b := result.Printers[i], result.Printers[j]
		left, right := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if left == right {
			return a.ID < b.ID
		}
		return left < right
	})
	return result
}

func copyValue[T any](v *T) *T {
	if v == nil {
		return nil
	}
	value := *v
	return &value
}
