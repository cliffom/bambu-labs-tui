package webserver

import (
	"encoding/json"

	"github.com/cliffom/bambu-tui/internal/session"
)

type (
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

func displaySlots(in []session.DisplaySlot) []slot {
	out := make([]slot, 0, len(in))
	for _, s := range in {
		out = append(out, slot(s))
	}
	return out
}

// The wire DTO is the HTTP allowlist. Session contracts carry neither JSON tags
// nor backend objects; encoding and artwork selection belong to this adapter.
func EncodeDisplay(state session.DisplayState) []byte {
	out := snapshot{At: state.At, Demo: state.Demo, Connected: state.Connected, Decisions: state.Decisions, Summary: summary(state.Summary), Printers: make([]printer, 0, len(state.Printers))}
	for _, p := range state.Printers {
		next := printer{ID: p.ID, Image: printerImage(p.Model), Name: p.Name, Model: p.Model, State: p.State, Online: p.Online, Connection: p.Connection, Fresh: p.Fresh, LastReport: p.LastReport, Job: p.Job, Stage: p.Stage, Progress: p.Progress, Remaining: p.Remaining, RemainingReported: p.RemainingReported, ETACurrent: p.ETACurrent, Layer: p.Layer, TotalLayers: p.TotalLayers, Bed: temperature(p.Bed), Chamber: temperature(p.Chamber), Nozzles: []nozzle{}, AMS: []ams{}, External: displaySlots(p.External), Conditions: []condition{}, Attention: p.Attention}
		for _, n := range p.Nozzles {
			next.Nozzles = append(next.Nozzles, nozzle{ID: n.ID, Active: n.Active, Temperature: temperature(n.Temperature)})
		}
		for _, a := range p.AMS {
			next.AMS = append(next.AMS, ams{ID: a.ID, Label: a.Label, Humidity: a.Humidity, Temperature: a.Temperature, Slots: displaySlots(a.Slots)})
		}
		for _, c := range p.Conditions {
			next.Conditions = append(next.Conditions, condition(c))
		}
		out.Printers = append(out.Printers, next)
	}
	data, _ := json.Marshal(out)
	return data
}

func (s *Server) PublishDisplay(state session.DisplayState) { s.Publish(EncodeDisplay(state)) }

func opaque(value string) string { return session.DisplayID(value) }
