package monitor

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
)

func Demo(ctx context.Context, out chan<- Event, refresh <-chan struct{}) {
	defer close(out)
	devices := []farm.Device{{ID: "DEMOH2D", Name: "H2D · Workshop", Model: "H2D", Online: true}, {ID: "DEMOX2D", Name: "X2D · Studio", Model: "X2D", Online: true}}
	if !emit(ctx, out, Event{Kind: EventDevices, Devices: devices}) {
		return
	}
	if !emit(ctx, out, Event{Kind: EventConnection, Connected: true}) {
		return
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	step := 0
	for {
		for i, d := range devices {
			state := "RUNNING"
			progress := (62 + step + i*9) % 100
			remaining := 125 - progress
			if i == 1 && step%20 >= 5 && step%20 < 8 {
				state = "PAUSE"
			}
			if i == 1 && step%20 >= 15 {
				state = "FINISH"
				progress = 100
				remaining = 0
			}
			trays := []any{map[string]any{"id": "0", "state": 3, "tray_type": "PLA", "tray_color": "B7E4C7FF", "remain": 68}, map[string]any{"id": "1", "state": 3, "tray_type": "PETG", "tray_color": "EF8354FF", "remain": 8}, map[string]any{"id": "2", "state": 3, "tray_type": "PLA-CF", "tray_color": "353B48FF", "remain": -1}, map[string]any{"id": "3", "state": 0}}
			payload := map[string]any{"print": map[string]any{"command": "push_status", "task_id": "demo", "gcode_state": state, "subtask_name": []string{"Gridfinity baseplate · plate 2", "Enclosure brackets · PETG"}[i], "mc_percent": progress, "mc_remaining_time": remaining, "layer_num": progress * 3, "total_layer_num": 300, "stg_cur": 0, "print_error": 0, "hms": []any{}, "device": map[string]any{"bed": map[string]any{"info": map[string]any{"temp": 60<<16 | 60}}, "ctc": map[string]any{"info": map[string]any{"temp": 35}}, "extruder": map[string]any{"state": 2, "info": []any{map[string]any{"id": 0, "temp": 220<<16 | 219, "snow": 0}, map[string]any{"id": 1, "temp": 180<<16 | 179, "snow": 1}}}}, "ams": map[string]any{"ams": []any{map[string]any{"id": "0", "temp": "28.4", "humidity_raw": "22", "tray": trays}}}, "vir_slot": []any{map[string]any{"id": "254", "tray_type": "TPU", "tray_color": "8899AAFF", "remain": 0}}}}
			addDemoFirmware(payload["print"].(map[string]any), i == 1)
			addDemoAMS(payload["print"].(map[string]any))
			if state == "PAUSE" {
				addDemoRunout(payload["print"].(map[string]any))
			}
			b, _ := json.Marshal(payload)
			if !emit(ctx, out, Event{Kind: EventReport, ID: d.ID, Payload: b}) {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			step++
		case <-refresh:
		}
	}
}

// addDemoAMS adds a second standard unit and AMS HT for inventory-browser navigation.
func addDemoAMS(report map[string]any) {
	ams := report["ams"].(map[string]any)
	units := ams["ams"].([]any)
	ams["ams"] = append(units,
		map[string]any{"id": "1", "temp": "27", "humidity_raw": "30", "tray": []any{
			map[string]any{"id": "0", "state": 3, "tray_type": "PLA", "tray_color": "D6CCA3FF", "remain": 92},
			map[string]any{"id": "1", "state": 0},
			map[string]any{"id": "2", "state": 3, "tray_type": "PETG", "tray_color": "4477AAFF", "remain": -1},
			map[string]any{"id": "3", "state": 3, "tray_type": "PLA", "tray_color": "FFFFFFFF", "remain": 13},
		}},
		map[string]any{"id": "128", "temp": "33", "humidity_raw": "11", "tray": []any{
			map[string]any{"id": "0", "state": 3, "tray_type": "PETG", "tray_color": "668C80FF", "remain": 64},
		}},
	)
}

// addDemoFirmware illustrates current and update-available OTA reports using synthetic versions.
func addDemoFirmware(report map[string]any, update bool) {
	flag, available := 2, "01.00.00.00"
	if update {
		flag, available = 1, "01.01.00.00"
	}
	report["upgrade_state"] = map[string]any{
		"status": "IDLE", "new_version_state": flag,
		"new_ver_list": []any{map[string]any{"name": "ota", "cur_ver": "01.00.00.00", "new_ver": available}},
	}
}

// addDemoRunout supplies a verified runout incident for the action-needed demo.
func addDemoRunout(report map[string]any) {
	report["stg_cur"] = 6
	report["print_error"] = 0x07008011
	report["hms"] = []any{map[string]any{"attr": 0x07002100, "code": 0x00020001}}
}
