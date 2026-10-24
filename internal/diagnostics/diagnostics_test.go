package diagnostics

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/buildinfo"
	"github.com/cliffom/bambu-tui/internal/farm"
)

// diagnosticState deliberately contains identifiers that must never enter an export.
func diagnosticState() (*farm.State, time.Time) {
	now := time.Now()
	s := farm.New(10)
	s.Cloud(true, "PRIVATE_ERROR https://private.invalid/?token=PRIVATE_TOKEN", now)
	s.Discover([]farm.Device{{ID: "PRIVATE_SERIAL", Name: "PRIVATE_PRINTER", Model: "H2D", Online: true}}, now)
	return s, now
}

// encoded builds a document without writing it and checks JSON encoding succeeds.
func encoded(t *testing.T, r *Recorder, s *farm.State, now time.Time) (Document, string) {
	t.Helper()
	doc, err := r.Build(s, "PRIVATE_SERIAL", now)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return doc, string(body)
}

// TestAdversarialNestedDataIsOmitted covers private values, field names, URLs, and malformed types.
func TestAdversarialNestedDataIsOmitted(t *testing.T) {
	s, now := diagnosticState()
	var r Recorder
	payload := []byte(`{"authorization":"PRIVATE_TOKEN","PRIVATE_EMAIL@example.com":{"nested":"PRIVATE_SECRET"},"print":{
"task_id":"PRIVATE_JOB_ID","subtask_name":"PRIVATE_JOB_NAME","gcode_file":"PRIVATE_FILE.3mf","gcode_state":"RUNNING","mc_percent":25,
"mc_remaining_time":"https://private.invalid/?token=PRIVATE_TOKEN","print_error":0,
"device":{"serial":"PRIVATE_SERIAL","bed":{"info":{"temp":{"PRIVATE_NESTED":"PRIVATE_SECRET"}}},"extruder":{"state":16,"info":[{"id":0,"temp":15728880,"snow":0,"url":"https://private.invalid/PRIVATE_SECRET"}]}},
"ams":{"ams":[{"id":"0","sn":"PRIVATE_AMS_SERIAL","temp":"28.5","tray":[{"id":"0","state":3,"tray_uuid":"PRIVATE_SPOOL","tray_type":"PRIVATE_CUSTOM_MATERIAL","tray_color":"PRIVATE_COLOR","remain":"PRIVATE_SECRET"}]}]},
"hms":[{"attr":50331904,"code":65543,"message":"PRIVATE_ERROR https://private.invalid/?token=PRIVATE_TOKEN"},{"attr":0,"code":{"PRIVATE_KEY":"PRIVATE_TOKEN"}}],
"unknown":{"PRIVATE_FIELD":"PRIVATE_TOKEN"}},"info":{"command":"get_version","module":[{"name":"ota","sw_ver":"01.02.03.04","sn":"PRIVATE_MODULE_SERIAL"}]}}`)
	if err := s.Report("PRIVATE_SERIAL", payload, now); err != nil {
		t.Fatal(err)
	}
	r.Record("PRIVATE_SERIAL", payload)
	doc, body := encoded(t, &r, s, now)
	for _, forbidden := range []string{"PRIVATE_", "private.invalid", "authorization", "task_id", "subtask_name", "gcode_file", "tray_uuid", "message"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("diagnostics leaked forbidden marker %q", forbidden)
		}
	}
	if doc.Printer.Alias != "printer-1" || doc.Printer.Model != "H2D" || doc.Printer.Firmware != "01.02.03.04" || len(doc.Printer.FaultCodes) == 0 {
		t.Fatal("useful model/firmware/fault metadata missing")
	}
	shape := doc.Recent[0].Structure.Fields["print"].Fields["mc_remaining_time"]
	if shape.Type != "string" || shape.Accepted {
		t.Fatal("invalid classified field lost its type/rejection evidence")
	}
	if doc.Recent[0].Structure.Omitted != 2 {
		t.Fatal("unknown root field names were not omitted/countable")
	}
	p := s.Printers["PRIVATE_SERIAL"]
	p.Model = "PRIVATE_MODEL"
	p.ModelCode = "PRIVATE_CODE"
	p.Firmware.Version = "PRIVATE_VERSION"
	p.Firmware.Available = "PRIVATE_URL"
	p.Faults = append(p.Faults, farm.Fault{Code: "PRIVATE_CODE", Text: "PRIVATE_SECRET"})
	_, body = encoded(t, &r, s, now)
	if strings.Contains(body, "PRIVATE_") {
		t.Fatal("normalized metadata bypassed the allowlist")
	}
}

// TestSyntheticFixturesRemainUseful preserves values/types needed for normalization regressions.
func TestSyntheticFixturesRemainUseful(t *testing.T) {
	for _, model := range []string{"h2d", "x2d"} {
		t.Run(model, func(t *testing.T) {
			s, now := diagnosticState()
			var r Recorder
			s.Printers["PRIVATE_SERIAL"].Model = strings.ToUpper(model)
			payload, err := os.ReadFile("../farm/testdata/" + model + ".json")
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Report("PRIVATE_SERIAL", payload, now); err != nil {
				t.Fatal(err)
			}
			r.Record("PRIVATE_SERIAL", payload)
			doc, body := encoded(t, &r, s, now)
			if strings.Contains(body, "synthetic-spool") || strings.Contains(body, "Fixture") || strings.Contains(body, "synthetic-"+model+"-job") {
				t.Fatal("fixture identifiers leaked")
			}
			fixture := farm.New(10)
			fixture.Cloud(true, "", now)
			fixture.Discover([]farm.Device{{ID: "TEST", Online: true}}, now)
			clean, err := json.Marshal(doc.Snapshot.Values)
			if err != nil {
				t.Fatal(err)
			}
			if err = fixture.Report("TEST", clean, now); err != nil {
				t.Fatal(err)
			}
			a, b := s.Printers["PRIVATE_SERIAL"], fixture.Printers["TEST"]
			if a.State != b.State || !reflect.DeepEqual(a.Nozzles, b.Nozzles) || !reflect.DeepEqual(a.AMS, b.AMS) || !reflect.DeepEqual(a.Progress, b.Progress) {
				t.Fatal("sanitized fixture lost classified normalization data")
			}
			if doc.Snapshot.Structure.Fields["print"].Fields["ams"].Type != "object" {
				t.Fatal("nested structure missing")
			}
		})
	}
}

// TestPartialMalformedAndBoundedReports retains only recent sanitized evidence for the selected printer.
func TestPartialMalformedAndBoundedReports(t *testing.T) {
	s, now := diagnosticState()
	var r Recorder
	for i := 0; i < recentLimit+3; i++ {
		r.Record("PRIVATE_SERIAL", []byte(`{"print":{"mc_percent":1}}`))
	}
	if len(r.recent) != recentLimit {
		t.Fatal("recorder unbounded")
	}
	r.Record("OTHER_PRIVATE_SERIAL", []byte(`{"print":{"mc_percent":42}}`))
	r.Record("PRIVATE_SERIAL", []byte(`{"print":{"bed_temper":55,"unknown":"PRIVATE_SECRET"}}`))
	r.Record("PRIVATE_SERIAL", []byte(`PRIVATE_TOKEN invalid JSON`))
	r.Record("PRIVATE_SERIAL", make([]byte, maxPayload+1))
	doc, body := encoded(t, &r, s, now)
	if len(doc.Recent) != recentLimit-1 || doc.Recent[len(doc.Recent)-1].Status != "too_large" || doc.Recent[len(doc.Recent)-2].Status != "invalid_json" || strings.Contains(body, "PRIVATE_") {
		t.Fatal("report bounds/selection/malformed-body protection failed")
	}
	lastValid := doc.Recent[len(doc.Recent)-3]
	if _, exists := lastValid.Values["print"].(map[string]any)["mc_percent"]; exists {
		t.Fatal("partial report was replaced by a merged snapshot")
	}
	if _, err := r.Build(s, "removed", now); err == nil {
		t.Fatal("removed printer exported")
	}
}

// TestShapeLimitsAreDeterministic prevents oversized arrays and map order from changing the export.
func TestShapeLimitsAreDeterministic(t *testing.T) {
	trays := make([]any, 100)
	for i := range trays {
		trays[i] = map[string]any{"id": "0", "remain": 8, "tray_uuid": "PRIVATE_SPOOL"}
	}
	units := make([]any, 100)
	for i := range units {
		units[i] = map[string]any{"id": "0", "temp": "28.5", "tray": trays}
	}
	value := map[string]any{"print": map[string]any{"ams": map[string]any{"ams": units}, "gcode_state": "RUNNING"}}
	first, err := json.Marshal(sanitizeReport(value))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		next, _ := json.Marshal(sanitizeReport(value))
		if string(next) != string(first) {
			t.Fatal("sanitizer depends on map iteration order")
		}
	}
	if len(first) > 200000 || strings.Contains(string(first), "PRIVATE_") {
		t.Fatal("bounded schema leaked or grew unexpectedly")
	}
}

// TestWriteIsPrivateAndExclusive covers existing files, symlinks, invalid JSON values, and blocked paths.
func TestWriteIsPrivateAndExclusive(t *testing.T) {
	s, now := diagnosticState()
	var r Recorder
	doc, _ := encoded(t, &r, s, now)
	root := t.TempDir()
	path := filepath.Join(root, "new", "export.json")
	if err := Write(path, doc); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("artifact permissions not private")
	}
	info, err = os.Stat(filepath.Dir(path))
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("new diagnostic directory not private")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = Write(path, doc); err == nil {
		t.Fatal("existing export overwritten")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("existing export changed")
	}
	link := filepath.Join(root, "symlink.json")
	if err = os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err = Write(link, doc); err == nil {
		t.Fatal("followed output symlink")
	}
	if err = Write(filepath.Join(path, "PRIVATE_PATH.json"), doc); err == nil || strings.Contains(err.Error(), "PRIVATE_PATH") {
		t.Fatal("failed export leaked path or suppressed error")
	}
	doc.Normalized["bad"] = math.NaN()
	invalid := filepath.Join(root, "invalid.json")
	if err = Write(invalid, doc); err == nil {
		t.Fatal("JSON encoding failure swallowed")
	}
	if _, err = os.Stat(invalid); !os.IsNotExist(err) {
		t.Fatal("invalid export left an artifact")
	}
}

// FuzzPrivateValuesNeverExported probes known-field type confusion and unclassified nested data.
func FuzzPrivateValuesNeverExported(f *testing.F) {
	for _, seed := range []string{"token", "https://host.invalid/?email=me@example.com", "\x1b]52;c;secret\a", "日本語"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 65536 {
			t.Skip()
		}
		secret := "PRIVATE_" + value
		s, now := diagnosticState()
		var r Recorder
		p := s.Printers["PRIVATE_SERIAL"]
		p.Raw = map[string]any{"gcode_state": secret, "mc_percent": secret, "PRIVATE_KEY_" + value: secret, "ams": map[string]any{"ams": []any{map[string]any{"id": secret, "tray": []any{map[string]any{"id": "0", "remain": secret, "tray_type": secret, "tray_color": secret, "tray_uuid": secret}}}}}, "hms": []any{map[string]any{"attr": 50331904, "code": 65543, "message": secret}}, "upgrade_state": map[string]any{"new_ver_list": []any{map[string]any{"name": secret, "cur_ver": secret, "new_ver": secret}}, "ota_new_version_number": secret}}
		payload, err := json.Marshal(map[string]any{"print": p.Raw})
		if err != nil {
			t.Fatal(err)
		}
		r.Record("PRIVATE_SERIAL", payload)
		_, body := encoded(t, &r, s, now)
		if strings.Contains(body, "PRIVATE_") {
			t.Fatal("private marker escaped the allowlist")
		}
	})
}

// TestMetadataIsValidatedAndSnapshotted prevents private build labels and live pointer reuse.
func TestMetadataIsValidatedAndSnapshotted(t *testing.T) {
	version, commit := buildinfo.Version, buildinfo.Commit
	defer func() { buildinfo.Version, buildinfo.Commit = version, commit }()
	buildinfo.Version = "PRIVATE_VERSION"
	buildinfo.Commit = "PRIVATE_COMMIT"
	s, now := diagnosticState()
	var r Recorder
	available := true
	s.Printers["PRIVATE_SERIAL"].Firmware = farm.Firmware{Version: "01.02.03.04", UpdateAvailable: &available}
	doc, body := encoded(t, &r, s, now)
	if strings.Contains(body, "PRIVATE_") || doc.App.Version != "unknown" || doc.App.Commit != "unknown" {
		t.Fatal("build metadata bypassed validation")
	}
	available = false
	if doc.Printer.UpdateAvailable == nil || !*doc.Printer.UpdateAvailable {
		t.Fatal("confirmed metadata snapshot borrowed a live pointer")
	}
}

// TestUnknownSignedFaultCodesRemainReproducible retains bounded wire codes without their messages.
func TestUnknownSignedFaultCodesRemainReproducible(t *testing.T) {
	s, now := diagnosticState()
	var r Recorder
	payload := []byte(`{"print":{"gcode_state":"PAUSE","print_error":-1,"hms":[],"message":"PRIVATE_ERROR"}}`)
	if err := s.Report("PRIVATE_SERIAL", payload, now); err != nil {
		t.Fatal(err)
	}
	r.Record("PRIVATE_SERIAL", payload)
	doc, body := encoded(t, &r, s, now)
	if doc.Snapshot.Values["print"].(map[string]any)["print_error"] != float64(-1) || len(doc.Printer.FaultCodes) != 1 || doc.Printer.FaultCodes[0] != "FFFF_FFFF" || strings.Contains(body, "PRIVATE_") {
		t.Fatal("unknown fault code lost its reproducible numeric form")
	}
}
