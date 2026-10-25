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
	p.versionReport(object(envelope[fieldInfo]), now)
	delta, ok := envelope[fieldPrint].(map[string]any)
	if !ok {
		return false, nil
	}
	if cmd := str(delta[fieldCommand]); cmd != "" && cmd != commandStatus {
		return false, nil
	}
	previous := p.State
	// A new job must not inherit the previous job's ETA/progress while its
	// initial report is still arriving.
	task := str(delta[fieldTaskID])
	newTask := task != "" && str(p.Raw[fieldTaskID]) != "" && task != str(p.Raw[fieldTaskID])
	restart := str(delta[fieldGcodeState]) == StateRunning && (previous == StateFinished || previous == StateFailed || previous == StateIdle)
	if newTask || restart {
		for _, k := range []string{fieldRemainingMinutes, fieldProgress, fieldLayerNum, fieldTotalLayerNum, fieldSubtaskName, fieldGcodeFile, fieldStage} {
			delete(p.Raw, k)
		}
		p.RemainingReported = time.Time{}
	}
	if _, ok := delta[fieldRemainingMinutes]; ok {
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
		if a, ok := v.([]any); ok && len(a) > 0 && (k == fieldAMS || k == fieldTray || k == fieldInfo || k == fieldVirSlot) {
			old, _ := dst[k].([]any)
			for _, item := range a {
				obj, ok := item.(map[string]any)
				if !ok {
					continue
				}
				id, hasID := obj[fieldID]
				if !hasID {
					continue
				}
				// Legacy firmware reports an empty tray as just its identity.
				if k == fieldTray && len(obj) == 1 {
					obj[fieldState] = float64(0)
				}
				found := false
				for _, existing := range old {
					m, ok := existing.(map[string]any)
					if ok && str(m[fieldID]) == str(id) {
						if k == fieldTray {
							if state := number(obj[fieldState]); state != nil && *state&spoolPresentMask == 0 {
								for key := range m {
									if key != fieldID {
										delete(m, key)
									}
								}
							}
						}
						// A replacement spool must not inherit the previous spool's estimate.
						for _, identity := range []string{fieldTrayUUID, fieldTagUID} {
							if v := str(obj[identity]); v != "" && v != str(m[identity]) {
								delete(m, fieldRemain)
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

func array(v any) []any { a, _ := v.([]any); return a }

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

func ptr(n int) *int { return &n }

func boolean(b bool) *bool { return &b }

func packed(v any) Temperature {
	n := number(v)
	if n == nil {
		return Temperature{}
	}
	return Temperature{ptr(*n & temperatureMask), ptr((*n >> temperatureBits) & temperatureMask)}
}
