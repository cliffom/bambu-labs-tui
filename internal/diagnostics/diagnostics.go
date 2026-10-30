// Package diagnostics builds narrow, allowlisted local troubleshooting artifacts.
package diagnostics

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/cliffom/bambustand/internal/buildinfo"
	"github.com/cliffom/bambustand/internal/farm"
)

var exportSchema = telemetrySchema()

const (
	recentLimit = 16
	maxPayload  = 1024 * 1024
)

type (
	// Report retains a fixture-ready allowlisted envelope and its classified structure.
	Report struct {
		Status    string         `json:"status"`
		Values    map[string]any `json:"values,omitempty"`
		Structure Shape          `json:"structure"`
	}

	sample struct {
		id     string
		report Report
	}

	// Recorder stores at most sixteen already-sanitized reports across the farm in memory.
	Recorder struct{ recent []sample }

	// App contains build/platform metadata without hostname, username, environment, or local paths.
	App struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		OS      string `json:"os"`
		Arch    string `json:"arch"`
	}

	// Printer contains only a per-export alias, classified metadata, link state, and fault codes.
	Printer struct {
		Alias             string   `json:"alias"`
		Model             string   `json:"model"`
		Firmware          string   `json:"firmware,omitempty"`
		AvailableFirmware string   `json:"available_firmware,omitempty"`
		UpdateAvailable   *bool    `json:"update_available,omitempty"`
		Connection        string   `json:"connection"`
		ReportAgeSeconds  *int64   `json:"report_age_seconds,omitempty"`
		FaultCodes        []string `json:"fault_codes"`
	}

	// Document is versioned and deliberately excludes credentials, identifiers, names, and free text.
	Document struct {
		SchemaVersion  int            `json:"schema_version"`
		App            App            `json:"app"`
		CloudConnected bool           `json:"cloud_connected"`
		Printer        Printer        `json:"printer"`
		Snapshot       Report         `json:"merged_snapshot"`
		Normalized     map[string]any `json:"normalized_print"`
		Recent         []Report       `json:"recent_reports"`
	}
)

// Record discards malformed bodies and sanitizes valid JSON immediately without writing files.
func (r *Recorder) Record(id string, payload []byte) {
	report := Report{Status: statusInvalidJSON, Structure: Shape{Type: typeUnavailable}}
	if len(payload) > maxPayload {
		report.Status = statusTooLarge
	} else {
		var value any
		if json.Unmarshal(payload, &value) == nil {
			report = sanitizeReport(value)
		}
	}
	r.recent = append(r.recent, sample{id, report})
	if len(r.recent) > recentLimit {
		r.recent = append([]sample(nil), r.recent[len(r.recent)-recentLimit:]...)
	}
}

// sanitizeReport applies the schema with a fixed node budget to bound diagnostic memory.
func sanitizeReport(value any) Report {
	budget := sanitizeNodeBudget
	clean, structure, ok := sanitize(value, exportSchema, &budget)
	report := Report{Status: statusSanitized, Structure: structure}
	if !ok {
		report.Status = statusUnsupportedEnvelope
		return report
	}
	report.Values, _ = clean.(map[string]any)
	return report
}

var (
	appVersionPattern = regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}(-(dev|rc[0-9]{1,3}|beta[0-9]{1,3}|alpha[0-9]{1,3}))?$`)
	commitPattern     = regexp.MustCompile(`^[a-fA-F0-9]{7,64}$`)
	faultPattern      = regexp.MustCompile(`^[0-9A-F]{4}(_[0-9A-F]{4}){1,3}$`)
)

// Build snapshots one printer; callers must explicitly request Write to create an artifact.
func (r *Recorder) Build(state *farm.State, id string, now time.Time) (Document, error) {
	p := state.Printers[id]
	if p == nil {
		return Document{}, errors.New("printer unavailable")
	}
	app := App{Version: "unknown", Commit: "unknown", OS: runtime.GOOS, Arch: runtime.GOARCH}
	if appVersionPattern.MatchString(buildinfo.Version) {
		app.Version = buildinfo.Version
	}
	if commitPattern.MatchString(buildinfo.Commit) {
		app.Commit = buildinfo.Commit
	}
	printer := Printer{Alias: "printer-1", Model: "Unknown", Connection: p.Connection(now, state.Connected), FaultCodes: []string{}}
	for _, model := range []string{p.Model, p.ModelCode} {
		if model == "H2D" || model == "X2D" {
			printer.Model = model
			break
		}
	}
	if firmwarePattern.MatchString(p.Firmware.Version) {
		printer.Firmware = p.Firmware.Version
	}
	if firmwarePattern.MatchString(p.Firmware.Available) {
		printer.AvailableFirmware = p.Firmware.Available
	}
	if printer.Firmware != "" {
		if p.Firmware.UpdateAvailable != nil {
			value := *p.Firmware.UpdateAvailable
			printer.UpdateAvailable = &value
		}
	}
	if !p.LastReport.IsZero() {
		age := max(int64(0), int64(now.Sub(p.LastReport)/time.Second))
		printer.ReportAgeSeconds = &age
	}
	for _, fault := range p.Faults {
		if faultPattern.MatchString(fault.Code) {
			printer.FaultCodes = append(printer.FaultCodes, fault.Code)
		}
	}
	sort.Strings(printer.FaultCodes)
	snapshot := sanitizeReport(map[string]any{"print": p.Raw})
	// Re-normalize the allowlisted envelope into a synthetic printer, avoiding any
	// private normalized fields or strings retained by the live printer model.
	fixture := farm.New(state.LowFilament)
	fixture.Cloud(true, "", now)
	fixture.Discover([]farm.Device{{ID: "TEST", Name: "Synthetic", Online: true}}, now)
	payload, err := json.Marshal(snapshot.Values)
	if err != nil {
		return Document{}, errors.New("diagnostic snapshot unavailable")
	}
	if err := fixture.Report("TEST", payload, now); err != nil {
		return Document{}, errors.New("diagnostic normalization unavailable")
	}
	n := fixture.Printers["TEST"]
	normalized := normalizedPrint(n)
	doc := Document{SchemaVersion: schemaVersion, App: app, CloudConnected: state.Connected, Printer: printer, Snapshot: snapshot, Normalized: normalized, Recent: []Report{}}
	for _, entry := range r.recent {
		if entry.id == id {
			doc.Recent = append(doc.Recent, entry.report)
		}
	}
	return doc, nil
}

// normalizedPrint uses explicit fields rather than serializing evolving farm structs.
func normalizedPrint(p *farm.Printer) map[string]any {
	nozzles := make([]map[string]any, 0, len(p.Nozzles))
	for _, n := range p.Nozzles {
		nozzles = append(nozzles, map[string]any{"id": n.ID, "temperature": normalizedTemperature(n.Temperature), "active": n.Active, "ams": n.AMS, "slot": n.Slot})
	}
	units := make([]map[string]any, 0, len(p.AMS))
	for _, unit := range p.AMS {
		units = append(units, map[string]any{"id": unit.ID, "humidity": unit.Humidity, "temperature": unit.Temperature, "slots": normalizedSlots(unit.Slots)})
	}
	return map[string]any{
		"state": safeState(p.State), "progress": p.Progress, "remaining_minutes": p.Remaining, "layer": p.Layer, "total_layers": p.TotalLayers,
		"bed": normalizedTemperature(p.Bed), "chamber": normalizedTemperature(p.Chamber), "nozzles": nozzles, "ams": units, "external": normalizedSlots(p.External),
	}
}

// normalizedTemperature emits only classified current and target values.
func normalizedTemperature(t farm.Temperature) map[string]any {
	return map[string]any{"current": t.Current, "target": t.Target}
}

// normalizedSlots excludes any future private fields added to the farm's slot model.
func normalizedSlots(slots []farm.Slot) []map[string]any {
	output := make([]map[string]any, 0, len(slots))
	for _, slot := range slots {
		output = append(output, map[string]any{"id": slot.ID, "material": slot.Material, "color": slot.Color, "present": slot.Present, "remaining": slot.Remaining, "nozzle_ids": slot.Nozzles})
	}
	return output
}

// safeState returns the same fixed state vocabulary permitted in report fixtures.
func safeState(state string) string {
	switch state {
	case farm.StateRunning, farm.StatePaused, farm.StateFailed, farm.StateFinished, farm.StateIdle, "PREPARE", "SLICING":
		return state
	default:
		return farm.StateUnknown
	}
}

// Destination previews a private cache location without creating directories or files.
func Destination(now time.Time) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", errors.New("diagnostic directory unavailable")
	}
	// Retain the original namespace for upgrade compatibility; see docs/user/usage.md.
	return filepath.Join(base, "bambu-tui", "diagnostics", "bambustand-diagnostics-"+now.UTC().Format("20060102T150405.000000000Z")+".json"), nil
}

// Write creates one private file exclusively, never overwriting a file or following a target symlink.
func Write(destination string, doc Document) (err error) {
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return errors.New("encode diagnostics")
	}
	if err = os.MkdirAll(filepath.Dir(destination), privateDirectoryMode); err != nil {
		return errors.New("create diagnostic directory")
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode)
	if err != nil {
		return errors.New("create diagnostic file")
	}
	defer func() {
		_ = file.Close()
		if err != nil {
			_ = os.Remove(destination)
		}
	}()
	if _, err = file.Write(append(body, '\n')); err != nil {
		return errors.New("write diagnostics")
	}
	if err = file.Sync(); err != nil {
		return errors.New("sync diagnostics")
	}
	if err = file.Close(); err != nil {
		return errors.New("close diagnostics")
	}
	return nil
}

// Included describes the fixed export scope shown before explicit confirmation.
func Included() string {
	return strings.Join([]string{"One printer, aliased as printer-1; H2D/X2D model and numeric firmware when recognized.", "App version/platform, connection state, fault codes, and classified telemetry values/types.", fmt.Sprintf("Merged snapshot plus this printer's samples from the last %d sanitized reports across the farm.", recentLimit), "Unknown fields/names, free-form messages, URLs, credentials, account/serial/job/spool identifiers are omitted.", "Files stay local. Inspect the JSON before voluntarily attaching it to an issue."}, "\n")
}
