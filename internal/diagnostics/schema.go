package diagnostics

import (
	"math"
	"regexp"
	"sort"
	"strconv"

	"github.com/cliffom/bambu-tui/internal/farm"
)

type (
	rule struct {
		kind     string
		min, max float64
		enum     []string
		pattern  *regexp.Regexp
		fields   map[string]*rule
		item     *rule
		limit    int
	}

	// Shape describes only classified field names and types; omitted names/values never escape.
	Shape struct {
		Type     string           `json:"type"`
		Accepted bool             `json:"accepted"`
		Fields   map[string]Shape `json:"fields,omitempty"`
		Items    []Shape          `json:"items,omitempty"`
		Omitted  int              `json:"omitted,omitempty"`
	}
)

var (
	decimal         = regexp.MustCompile(`^-?[0-9]{1,10}(\.[0-9]{1,3})?$`)
	colorPattern    = regexp.MustCompile(`^[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?$`)
	firmwarePattern = regexp.MustCompile(`^[0-9]{1,4}(\.[0-9]{1,4}){3}$`)
)

func numeric(min, max float64) *rule { return &rule{kind: typeNumber, min: min, max: max} }

func enumeration(values ...string) *rule { return &rule{kind: ruleEnum, enum: values} }

func object(fields map[string]*rule) *rule { return &rule{kind: typeObject, fields: fields} }

func array(item *rule, limit int) *rule { return &rule{kind: typeArray, item: item, limit: limit} }

// telemetrySchema is an allowlist; unknown keys, messages, paths, URLs and identifiers are absent.
func telemetrySchema() *rule {
	version := &rule{kind: rulePattern, pattern: firmwarePattern}
	word := numeric(0, math.MaxUint32)
	faultWord := numeric(math.MinInt32, math.MaxUint32)
	id := numeric(0, maxByteValue)
	temperature := numeric(minTemperature, maxTemperature)
	material := enumeration("Empty", "PLA", "PETG", "ABS", "ASA", "TPU", "PA", "PC", "PVA", "PET", "PLA-CF", "PETG-CF", "PA-CF", "PA6-CF", "PAHT-CF", "PET-CF")
	tray := object(map[string]*rule{"id": id, "state": word, "tray_type": material, "tray_color": {kind: rulePattern, pattern: colorPattern}, "remain": numeric(-1, farm.MaxPercent)})
	unit := object(map[string]*rule{"id": id, "temp": numeric(0, maxAMSTemperature), "humidity_raw": numeric(0, farm.MaxPercent), "humidity": numeric(0, maxHumidityLevel), "tray": array(tray, farm.SlotsPerAMS)})
	packed := object(map[string]*rule{"temp": word})
	module := object(map[string]*rule{"name": enumeration("ota"), "sw_ver": version, "cur_ver": version, "new_ver": version})
	print := object(map[string]*rule{
		"command": enumeration("push_status", "pushall"), "gcode_state": enumeration(farm.StateRunning, farm.StatePaused, farm.StateFailed, farm.StateFinished, farm.StateIdle, "PREPARE", "SLICING", farm.StateUnknown),
		"mc_percent": numeric(0, farm.MaxPercent), "mc_remaining_time": numeric(-1, farm.MaxRemainingMinutes), "layer_num": numeric(0, farm.MaxLayers), "total_layer_num": numeric(0, farm.MaxLayers), "stg_cur": numeric(0, maxByteValue),
		"bed_temper": temperature, "bed_target_temper": temperature, "chamber_temper": temperature, "chamber_target_temper": temperature, "nozzle_temper": temperature, "nozzle_target_temper": temperature,
		"print_error": faultWord, "hms": array(object(map[string]*rule{"attr": faultWord, "code": faultWord}), maxHMSItems),
		"device": object(map[string]*rule{
			"bed_temp": word, "bed": object(map[string]*rule{"info": packed}), "ctc": object(map[string]*rule{"info": packed}),
			"extruder": object(map[string]*rule{"state": word, "info": array(object(map[string]*rule{"id": numeric(0, 1), "temp": word, "snow": word}), nozzleCount)}),
		}),
		"ams": object(map[string]*rule{"tray_now": id, "ams": array(unit, maxAMSItems)}), "vir_slot": array(tray, maxExternalSlots), "vt_tray": tray,
		"upgrade_state": object(map[string]*rule{"new_version_state": numeric(0, 2), "ota_new_version_number": version, "status": enumeration("IDLE", "UPGRADING", "DOWNLOADING", "PREPARING", "INSTALLING", "FAILED", "SUCCESS"), "new_ver_list": array(module, maxUpgradeModules)}),
	})
	return object(map[string]*rule{"print": print, "info": object(map[string]*rule{"command": enumeration("get_version"), "result": enumeration("success", "fail", "failed"), "module": array(module, maxModules)})})
}

// valueType returns a fixed vocabulary rather than formatting untrusted values or types.
func valueType(value any) string {
	switch value.(type) {
	case nil:
		return typeNull
	case map[string]any:
		return typeObject
	case []any:
		return typeArray
	case string:
		return typeString
	case float64:
		return typeNumber
	case bool:
		return typeBoolean
	default:
		return typeUnsupported
	}
}

// sanitize preserves classified values and bounded structural evidence, never arbitrary text.
func sanitize(value any, r *rule, budget *int) (any, Shape, bool) {
	shape := Shape{Type: valueType(value)}
	if *budget <= 0 {
		shape.Omitted = 1
		return nil, shape, false
	}
	*budget -= 1
	if value == nil {
		shape.Accepted = true
		return nil, shape, true
	}
	switch r.kind {
	case typeObject:
		input, ok := value.(map[string]any)
		if !ok {
			break
		}
		output := map[string]any{}
		shape.Fields = map[string]Shape{}
		shape.Omitted = len(input)
		keys := make([]string, 0, len(r.fields))
		for key := range r.fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := r.fields[key]
			if raw, present := input[key]; present {
				shape.Omitted--
				clean, childShape, keep := sanitize(raw, child, budget)
				shape.Fields[key] = childShape
				if keep {
					output[key] = clean
				}
			}
		}
		shape.Accepted = true
		return output, shape, true
	case typeArray:
		input, ok := value.([]any)
		if !ok {
			break
		}
		count := min(len(input), r.limit)
		output := make([]any, 0, count)
		shape.Omitted = len(input) - count
		for _, raw := range input[:count] {
			clean, childShape, _ := sanitize(raw, r.item, budget)
			output = append(output, clean)
			shape.Items = append(shape.Items, childShape)
		}
		shape.Accepted = true
		return output, shape, true
	case typeNumber:
		n, ok := value.(float64)
		if text, isString := value.(string); isString && decimal.MatchString(text) {
			var err error
			n, err = strconv.ParseFloat(text, 64)
			ok = err == nil
		}
		if ok && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= r.min && n <= r.max {
			shape.Accepted = true
			return value, shape, true
		}
	case ruleEnum:
		if text, ok := value.(string); ok {
			for _, allowed := range r.enum {
				if text == allowed {
					shape.Accepted = true
					return text, shape, true
				}
			}
		}
	case rulePattern:
		if text, ok := value.(string); ok && r.pattern.MatchString(text) {
			shape.Accepted = true
			return text, shape, true
		}
	}
	return nil, shape, false
}
