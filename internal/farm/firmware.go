package farm

import (
	"strconv"
	"strings"
	"time"
)

// Firmware stores printer OTA metadata separately from print-status freshness.
type Firmware struct {
	Version, Available              string
	UpdateAvailable                 *bool
	VersionReported, StatusReported time.Time
}

// firmwareVersion accepts the four numeric components reported for printer OTA.
func firmwareVersion(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return ""
	}
	nonzero := false
	for _, part := range parts {
		if part == "" || len(part) > 4 {
			return ""
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return ""
			}
		}
		n, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return ""
		}
		nonzero = nonzero || n != 0
	}
	if !nonzero {
		return ""
	}
	return s
}

// setFirmwareVersion invalidates an update assessment when the installed version changes.
func (p *Printer) setFirmwareVersion(version string, now time.Time) {
	if version == "" {
		return
	}
	if p.Firmware.Version != "" && p.Firmware.Version != version {
		p.Firmware.UpdateAvailable, p.Firmware.Available = nil, ""
		p.Firmware.StatusReported = time.Time{}
	}
	p.Firmware.Version, p.Firmware.VersionReported = version, now
	if p.Firmware.Available != "" {
		p.Firmware.UpdateAvailable = boolean(!sameFirmwareVersion(p.Firmware.Available, version))
	}
}

// versionReport selects only the printer OTA module, never controller/AMS versions.
func (p *Printer) versionReport(info map[string]any, now time.Time) {
	if str(info["command"]) != "get_version" {
		return
	}
	if result := str(info["result"]); result != "" && !strings.EqualFold(result, "success") {
		return
	}
	for _, entry := range array(info["module"]) {
		module := object(entry)
		if str(module["name"]) == "ota" {
			p.setFirmwareVersion(firmwareVersion(module["sw_ver"]), now)
			return
		}
	}
}

// firmwareReport consumes explicit upgrade metadata and preserves omitted fields.
func (p *Printer) firmwareReport(delta map[string]any, now time.Time) {
	value, present := delta["upgrade_state"]
	if !present {
		return
	}
	update := object(value)
	if update == nil {
		p.Firmware.UpdateAvailable, p.Firmware.Available = nil, ""
		p.Firmware.StatusReported = time.Time{}
		return
	}
	_, hasState := update["new_version_state"]
	_, hasList := update["new_ver_list"]
	_, hasLegacy := update["ota_new_version_number"]
	_, hasStatus := update["status"]
	if !hasState && !hasList && !hasLegacy {
		if status := str(update["status"]); hasStatus && status != "" && status != "IDLE" {
			p.Firmware.UpdateAvailable, p.Firmware.Available = nil, ""
			p.Firmware.StatusReported = now
		}
		return
	}
	// Use merged metadata for partial reports, but only a newly reported OTA
	// entry can override an installed version from a later get_version response.
	for _, entry := range array(update["new_ver_list"]) {
		module := object(entry)
		if str(module["name"]) == "ota" {
			p.setFirmwareVersion(firmwareVersion(module["cur_ver"]), now)
		}
	}
	state := object(p.Raw["upgrade_state"])
	available := p.Firmware.Available
	p.Firmware.UpdateAvailable, p.Firmware.Available = nil, ""
	p.Firmware.StatusReported = now
	if status := str(state["status"]); status != "" && status != "IDLE" {
		return // An in-progress/failed/finished upgrade needs a fresh assessment.
	}
	flag := str(state["new_version_state"])
	if flag == "2" {
		p.Firmware.UpdateAvailable = boolean(false)
		return
	}
	if flag != "1" {
		return
	}
	if hasList {
		available = ""
		for _, entry := range array(update["new_ver_list"]) {
			module := object(entry)
			if str(module["name"]) == "ota" {
				available = firmwareVersion(module["new_ver"])
				break
			}
		}
	}
	if hasLegacy && (!hasList || available == "") {
		available = firmwareVersion(update["ota_new_version_number"])
	}
	p.Firmware.Available = available
	if available != "" && p.Firmware.Version != "" {
		p.Firmware.UpdateAvailable = boolean(!sameFirmwareVersion(available, p.Firmware.Version))
	}
}

// sameFirmwareVersion compares numeric components without assuming fixed padding.
func sameFirmwareVersion(a, b string) bool {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	if len(left) != 4 || len(right) != 4 {
		return false
	}
	for i := range left {
		if strings.TrimLeft(left[i], "0") != strings.TrimLeft(right[i], "0") {
			return false
		}
	}
	return true
}
