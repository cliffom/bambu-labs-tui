package webserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	snapshot struct {
		Decisions string    `json:"decisions"`
		At        string    `json:"at"`
		Demo      bool      `json:"demo"`
		Connected bool      `json:"connected"`
		Summary   summary   `json:"summary"`
		Printers  []printer `json:"printers"`
	}
	summary struct {
		Total       int `json:"total"`
		Printing    int `json:"printing"`
		Attention   int `json:"attention"`
		Unavailable int `json:"unavailable"`
	}
	printer struct {
		ID                string      `json:"id"`
		Image             string      `json:"image"`
		Name              string      `json:"name"`
		Model             string      `json:"model"`
		State             string      `json:"state"`
		Online            bool        `json:"online"`
		Connection        string      `json:"connection"`
		Fresh             bool        `json:"fresh"`
		LastReport        string      `json:"last_report"`
		Job               string      `json:"job"`
		Stage             string      `json:"stage"`
		Progress          *int        `json:"progress"`
		Remaining         *int        `json:"remaining"`
		RemainingReported string      `json:"remaining_reported"`
		ETACurrent        bool        `json:"eta_current"`
		Layer             *int        `json:"layer"`
		TotalLayers       *int        `json:"total_layers"`
		Bed               temperature `json:"bed"`
		Chamber           temperature `json:"chamber"`
		Nozzles           []nozzle    `json:"nozzles"`
		AMS               []ams       `json:"ams"`
		External          []slot      `json:"external"`
		Conditions        []condition `json:"conditions"`
		Attention         bool        `json:"attention"`
	}
	temperature struct {
		Current *int `json:"current"`
		Target  *int `json:"target"`
	}
	nozzle struct {
		ID          int         `json:"id"`
		Active      bool        `json:"active"`
		Temperature temperature `json:"temperature"`
	}
	slot struct {
		ID        int    `json:"id"`
		Material  string `json:"material"`
		Color     string `json:"color"`
		Present   *bool  `json:"present"`
		Remaining *int   `json:"remaining"`
	}
	ams struct {
		ID          int    `json:"id"`
		Label       string `json:"label"`
		Humidity    *int   `json:"humidity"`
		Temperature *int   `json:"temperature"`
		Slots       []slot `json:"slots"`
	}
	condition struct {
		ID       string `json:"id"`
		Level    string `json:"level"`
		Text     string `json:"text"`
		Code     string `json:"code"`
		Since    string `json:"since"`
		Reviewed bool   `json:"reviewed"`
		Hidden   bool   `json:"hidden"`
		Guidance string `json:"guidance"`
	}
)

func opaque(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func timestamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func temp(t farm.Temperature) temperature { return temperature{Current: t.Current, Target: t.Target} }

func slots(source []farm.Slot) []slot {
	result := make([]slot, 0, len(source))
	for _, s := range source {
		result = append(result, slot{ID: s.ID, Material: s.Material, Color: s.Color, Present: s.Present, Remaining: s.Remaining})
	}
	return result
}

// SnapshotJSON must run on the reducer, the sole owner of farm state. The DTO
// explicitly excludes Raw, device serials, protocol job IDs, account/session
// information, credentials, storage paths, and backend errors. Its JSON contains
// only finite primitive values; no mutable farm objects leave the reducer.
func SnapshotJSON(state *farm.State, now time.Time, demo bool, decisions ...Decisions) []byte {
	result := snapshot{At: timestamp(now), Demo: demo, Connected: state.Connected, Printers: make([]printer, 0, len(state.Printers))}
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
		out := printer{ID: opaque(id), Image: printerImage(model), Name: p.Name, Model: model, State: p.State, Online: p.Online, Connection: p.Connection(now, state.Connected), Fresh: p.TelemetryCurrent(now, state.Connected), LastReport: timestamp(p.LastReport), Job: p.Job, Stage: p.Stage, Progress: p.Progress, Remaining: p.Remaining, RemainingReported: timestamp(p.RemainingReported), ETACurrent: p.ETACurrent(now, state.Connected), Layer: p.Layer, TotalLayers: p.TotalLayers, Bed: temp(p.Bed), Chamber: temp(p.Chamber), Nozzles: []nozzle{}, AMS: []ams{}, External: slots(p.External), Conditions: []condition{}, Attention: attention[id]}
		for _, n := range p.Nozzles {
			out.Nozzles = append(out.Nozzles, nozzle{ID: n.ID, Active: n.Active, Temperature: temp(n.Temperature)})
		}
		for _, unit := range p.AMS {
			out.AMS = append(out.AMS, ams{ID: unit.ID, Label: farm.AMSLabel(unit.ID), Humidity: unit.Humidity, Temperature: unit.Temperature, Slots: slots(unit.Slots)})
		}
		for key, c := range state.Active {
			owner, _, _ := strings.Cut(key, "/")
			if owner != id {
				continue
			}
			identity := opaque(key + timestamp(c.Since))
			if local.Targets != nil {
				identity = local.Targets[key]
			}
			out.Conditions = append(out.Conditions, condition{ID: identity, Level: c.Level, Text: c.Text, Code: c.Code, Since: timestamp(c.Since), Reviewed: c.Acknowledged || c.Dismissed, Hidden: c.Dismissed, Guidance: guidance[key]})
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
	data, _ := json.Marshal(result) // The allowlist contains only JSON-safe primitives.
	return data
}
