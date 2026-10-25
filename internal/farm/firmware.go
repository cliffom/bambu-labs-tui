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
	if len(parts) != firmwarePartCount {
		return ""
	}
	nonzero := false
	for _, part := range parts {
		if part == "" || len(part) > maxFirmwarePartDigits {
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
	if str(info[fieldCommand]) != versionCommand {
		return
	}
	if result := str(info[fieldResult]); result != "" && !strings.EqualFold(result, versionSuccess) {
		return
	}
	for _, entry := range array(info[fieldModule]) {
		module := object(entry)
		if str(module[fieldName]) == otaModule {
			p.setFirmwareVersion(firmwareVersion(module[fieldSwVer]), now)
			return
		}
	}
}

// firmwareReport consumes explicit upgrade metadata and preserves omitted fields.
func (p *Printer) firmwareReport(delta map[string]any, now time.Time) {
	value, present := delta[fieldUpgradeState]
	if !present {
		return
	}
	update := object(value)
	if update == nil {
		p.Firmware.UpdateAvailable, p.Firmware.Available = nil, ""
		p.Firmware.StatusReported = time.Time{}
		return
	}
	_, hasState := update[fieldNewVersionState]
	_, hasList := update[fieldNewVerList]
	_, hasLegacy := update[fieldOTANewVersionNumber]
	_, hasStatus := update[fieldStatus]
	if !hasState && !hasList && !hasLegacy {
		if status := str(update[fieldStatus]); hasStatus && status != "" && status != firmwareUpgradeIdle {
			p.Firmware.UpdateAvailable, p.Firmware.Available = nil, ""
			p.Firmware.StatusReported = now
		}
		return
	}
	// Use merged metadata for partial reports, but only a newly reported OTA
	// entry can override an installed version from a later get_version response.
	for _, entry := range array(update[fieldNewVerList]) {
		module := object(entry)
		if str(module[fieldName]) == otaModule {
			p.setFirmwareVersion(firmwareVersion(module[fieldCurVer]), now)
		}
	}
	state := object(p.Raw[fieldUpgradeState])
	available := p.Firmware.Available
	p.Firmware.UpdateAvailable, p.Firmware.Available = nil, ""
	p.Firmware.StatusReported = now
	if status := str(state[fieldStatus]); status != "" && status != firmwareUpgradeIdle {
		return // An in-progress/failed/finished upgrade needs a fresh assessment.
	}
	flag := str(state[fieldNewVersionState])
	if flag == upgradeCurrent {
		p.Firmware.UpdateAvailable = boolean(false)
		return
	}
	if flag != upgradeAvailable {
		return
	}
	if hasList {
		available = ""
		for _, entry := range array(update[fieldNewVerList]) {
			module := object(entry)
			if str(module[fieldName]) == otaModule {
				available = firmwareVersion(module[fieldNewVer])
				break
			}
		}
	}
	if hasLegacy && (!hasList || available == "") {
		available = firmwareVersion(update[fieldOTANewVersionNumber])
	}
	p.Firmware.Available = available
	if available != "" && p.Firmware.Version != "" {
		p.Firmware.UpdateAvailable = boolean(!sameFirmwareVersion(available, p.Firmware.Version))
	}
}

// sameFirmwareVersion compares numeric components without assuming fixed padding.
func sameFirmwareVersion(a, b string) bool {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	if len(left) != firmwarePartCount || len(right) != firmwarePartCount {
		return false
	}
	for i := range left {
		if strings.TrimLeft(left[i], "0") != strings.TrimLeft(right[i], "0") {
			return false
		}
	}
	return true
}
