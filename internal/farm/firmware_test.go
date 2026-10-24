package farm

import (
	"testing"
	"time"
)

// TestFirmwareInfoDoesNotRefreshPrintTelemetry selects OTA without concealing stale reports.
func TestFirmwareInfoDoesNotRefreshPrintTelemetry(t *testing.T) {
	s := New(10)
	now := time.Now()
	s.Discover([]Device{{ID: "TEST", Online: true}}, now)
	info := `{"info":{"command":"get_version","result":"SUCCESS","module":[{"name":"ams/0","sw_ver":"99.00.00.00"},{"name":"ota","sw_ver":"01.02.03.00"}]}}`
	if err := s.Report("TEST", []byte(info), now); err != nil {
		t.Fatal(err)
	}
	p := s.Printers["TEST"]
	if p.Firmware.Version != "01.02.03.00" || !p.LastReport.IsZero() || !p.Stale(now) {
		t.Fatal("wrong module version selected or info reply refreshed print telemetry")
	}
	report(t, s, `{"mc_percent":50}`, now)
	if p.Firmware.Version != "01.02.03.00" {
		t.Fatal("partial print report discarded firmware version")
	}
	report(t, s, `{"upgrade_state":{"new_version_state":2}}`, now)
	report(t, s, `{"mc_percent":51,"upgrade_state":{"progress":"0"}}`, now.Add(time.Minute))
	if !p.Firmware.StatusReported.Equal(now) {
		t.Fatal("unrelated progress report refreshed the firmware assessment")
	}
	if err := s.Report("TEST", []byte(info), now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !p.Stale(now.Add(time.Hour)) {
		t.Fatal("get_version concealed stale printing data")
	}
}

// TestFirmwareUpdateMetadata distinguishes printer OTA updates from component updates.
func TestFirmwareUpdateMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, data, version, available string
		flag                           int // -1 unknown, 0 current, 1 update available
	}{
		{"current", `{"new_version_state":2}`, "", "", 0},
		{"OTA update", `{"new_version_state":1,"new_ver_list":[{"name":"ams/0","cur_ver":"9.0.0.0","new_ver":"10.0.0.0"},{"name":"ota","cur_ver":"01.00.00.00","new_ver":"01.01.00.00"}]}`, "01.00.00.00", "01.01.00.00", 1},
		{"legacy OTA", `{"new_version_state":1,"ota_new_version_number":"01.01.00.00"}`, "", "01.01.00.00", 1},
		{"component only", `{"new_version_state":1,"new_ver_list":[{"name":"ams/0","cur_ver":"1.0.0.0","new_ver":"2.0.0.0"}]}`, "", "", -1},
		{"OTA unchanged", `{"new_version_state":1,"new_ver_list":[{"name":"ota","cur_ver":"01.00.00.00","new_ver":"1.0.0.0"}]}`, "01.00.00.00", "1.0.0.0", 0},
		{"unrecognized flag", `{"new_version_state":7}`, "", "", -1},
		{"fractional flag", `{"new_version_state":2.5}`, "", "", -1},
		{"missing target", `{"new_version_state":1}`, "", "", -1},
		{"invalid target", `{"new_version_state":1,"ota_new_version_number":"Unknown"}`, "", "", -1},
		{"upgrading", `{"new_version_state":2,"status":"UPGRADING"}`, "", "", -1},
		{"null", `null`, "", "", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(10)
			now := time.Now()
			s.Discover([]Device{{ID: "TEST", Online: true}}, now)
			p := s.Printers["TEST"]
			p.setFirmwareVersion("01.00.00.00", now)
			report(t, s, `{"upgrade_state":`+tc.data+`}`, now)
			f := p.Firmware
			if tc.version != "" && f.Version != tc.version || f.Available != tc.available {
				t.Fatalf("wrong OTA versions: %+v", f)
			}
			if tc.flag == -1 {
				if f.UpdateAvailable != nil {
					t.Fatal("unknown status was treated as confirmed")
				}
			} else if f.UpdateAvailable == nil || *f.UpdateAvailable != (tc.flag == 1) {
				t.Fatalf("wrong update status: %+v", f)
			}
		})
	}
}

// TestFirmwareOrderingAndInvalidation handles startup ordering and clears obsolete targets.
func TestFirmwareOrderingAndInvalidation(t *testing.T) {
	s := New(10)
	now := time.Now()
	s.Discover([]Device{{ID: "TEST", Online: true}}, now)
	p := s.Printers["TEST"]
	report(t, s, `{"upgrade_state":{"new_version_state":2}}`, now)
	p.versionReport(map[string]any{"command": "get_version", "module": []any{map[string]any{"name": "ota", "sw_ver": "01.00.00.00"}}}, now)
	if p.Firmware.UpdateAvailable == nil || *p.Firmware.UpdateAvailable {
		t.Fatal("version reply lost a preceding no-update assessment")
	}
	p.setFirmwareVersion("02.00.00.00", now)
	report(t, s, `{"upgrade_state":{"status":"IDLE"}}`, now)
	if p.Firmware.UpdateAvailable != nil {
		t.Fatal("changed installation inherited an obsolete current assessment")
	}
	report(t, s, `{"upgrade_state":{"new_version_state":1,"ota_new_version_number":"03.00.00.00"}}`, now)
	report(t, s, `{"upgrade_state":{"new_version_state":2}}`, now)
	report(t, s, `{"upgrade_state":{"new_version_state":1}}`, now)
	if p.Firmware.UpdateAvailable != nil || p.Firmware.Available != "" {
		t.Fatal("old target leaked into a later update notification")
	}
	report(t, s, `{"upgrade_state":{"new_version_state":1,"ota_new_version_number":"03.00.00.00"}}`, now)
	report(t, s, `{"upgrade_state":{"new_ver_list":[{"name":"ams/0","new_ver":"02.00.00.00"}]}}`, now)
	if p.Firmware.UpdateAvailable != nil || p.Firmware.Available != "" {
		t.Fatal("component snapshot borrowed a stale printer target")
	}
	for _, invalid := range []any{nil, "", "unknown", "00.00.00.00", "1.2.3", "1.2.3.x", "1.2.3.4\x1b", 1234} {
		if firmwareVersion(invalid) != "" {
			t.Fatalf("invalid version accepted: %v", invalid)
		}
	}
}
