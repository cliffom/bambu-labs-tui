package farm

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"
)

// applyReport interprets wire data without emitting farm conditions or history.
// Firmware-only responses do not count as fresh print telemetry.
func (p *Printer) applyReport(payload []byte, now time.Time) (bool, error) {
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return false, fmt.Errorf("invalid status JSON")
	}
	p.versionReport(object(envelope["info"]), now)
	delta, ok := envelope["print"].(map[string]any)
	if !ok {
		return false, nil
	}
	if cmd := str(delta["command"]); cmd != "" && cmd != "push_status" {
		return false, nil
	}
	previous := p.State
	// A new job must not inherit the previous job's ETA/progress while its
	// initial report is still arriving.
	task := str(delta["task_id"])
	newTask := task != "" && str(p.Raw["task_id"]) != "" && task != str(p.Raw["task_id"])
	restart := str(delta["gcode_state"]) == "RUNNING" && (previous == "FINISH" || previous == "FAILED" || previous == "IDLE")
	if newTask || restart {
		for _, k := range []string{"mc_remaining_time", "mc_percent", "layer_num", "total_layer_num", "subtask_name", "gcode_file", "stg_cur"} {
			delete(p.Raw, k)
		}
		p.RemainingReported = time.Time{}
	}
	if _, ok := delta["mc_remaining_time"]; ok {
		p.RemainingReported = now
	}
	merge(p.Raw, delta)
	p.firmwareReport(delta, now)
	p.LastReport, p.Online = now, true
	p.normalize()
	return true, nil
}

// merge preserves omitted properties and identity-bearing arrays; explicit empty
// arrays clear a collection. Fault lists are snapshots rather than deltas.
func merge(dst, src map[string]any) {
	for k, v := range src {
		if obj, ok := v.(map[string]any); ok {
			old, _ := dst[k].(map[string]any)
			if old == nil {
				old = map[string]any{}
			}
			merge(old, obj)
			dst[k] = old
			continue
		}
		if a, ok := v.([]any); ok && len(a) > 0 && (k == "ams" || k == "tray" || k == "info" || k == "vir_slot") {
			old, _ := dst[k].([]any)
			for _, item := range a {
				obj, ok := item.(map[string]any)
				if !ok {
					continue
				}
				id, hasID := obj["id"]
				if !hasID {
					continue
				}
				// Legacy firmware reports an empty tray as just its identity.
				if k == "tray" && len(obj) == 1 {
					obj["state"] = float64(0)
				}
				found := false
				for _, existing := range old {
					m, ok := existing.(map[string]any)
					if ok && str(m["id"]) == str(id) {
						if k == "tray" {
							if state := number(obj["state"]); state != nil && *state&1 == 0 {
								for key := range m {
									if key != "id" {
										delete(m, key)
									}
								}
							}
						}
						// A replacement spool must not inherit the previous spool's estimate.
						for _, identity := range []string{"tray_uuid", "tag_uid"} {
							if v := str(obj[identity]); v != "" && v != str(m[identity]) {
								delete(m, "remain")
							}
						}
						merge(m, obj)
						found = true
						break
					}
				}
				if !found {
					m := map[string]any{}
					merge(m, obj)
					old = append(old, m)
				}
			}
			dst[k] = old
			continue
		}
		dst[k] = v
	}
}

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func array(v any) []any           { a, _ := v.([]any); return a }
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func number(v any) *int {
	var n float64
	var err error
	switch x := v.(type) {
	case float64:
		n = x
	case string:
		n, err = strconv.ParseFloat(x, 64)
	default:
		return nil
	}
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n > math.MaxUint32 || n < math.MinInt32 {
		return nil
	}
	i := int(n)
	return &i
}
func bounded(v any, min, max int) *int {
	n := number(v)
	if n == nil || *n < min || *n > max {
		return nil
	}
	return n
}
func ptr(n int) *int       { return &n }
func boolean(b bool) *bool { return &b }
func packed(v any) Temperature {
	n := number(v)
	if n == nil {
		return Temperature{}
	}
	return Temperature{ptr(*n & 0xffff), ptr((*n >> 16) & 0xffff)}
}
