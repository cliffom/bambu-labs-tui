package config

// AlertField identifies one independently inherited alert preference.
type AlertField uint8

const (
	DesktopNotifications AlertField = iota
	NotificationSound
	CompletedPrints
	AutomaticModals
	Filament
	Extrusion
	PrinterFaults
	FailedPrints
	UnknownPauses
	AlertFieldCount
)

func (r *AlertRules) fields() []*bool {
	return []*bool{&r.DesktopNotifications, &r.NotificationSound, &r.CompletedPrints, &r.AutomaticModals, &r.Filament, &r.Extrusion, &r.PrinterFaults, &r.FailedPrints, &r.UnknownPauses}
}

func (o *AlertOverride) fields() []**bool {
	return []**bool{&o.DesktopNotifications, &o.NotificationSound, &o.CompletedPrints, &o.AutomaticModals, &o.Filament, &o.Extrusion, &o.PrinterFaults, &o.FailedPrints, &o.UnknownPauses}
}

func (r AlertRules) Value(field AlertField) bool { return *r.fields()[field] }

func (r *AlertRules) Set(field AlertField, value bool) { *r.fields()[field] = value }

func (o AlertOverride) Value(field AlertField) *bool { return *o.fields()[field] }

func (o *AlertOverride) Set(field AlertField, value *bool) {
	if value != nil {
		copy := *value
		value = &copy
	}
	*o.fields()[field] = value
}

// Clone isolates maps and optional values from reducer mutations and async saves.
func (c Config) Clone() Config {
	c.Alerts.Printers = cloneOverrides(c.Alerts.Printers)
	return c
}

func cloneOverrides(source map[string]AlertOverride) map[string]AlertOverride {
	if source == nil {
		return nil
	}
	result := make(map[string]AlertOverride, len(source))
	for id, override := range source {
		var copy AlertOverride
		for field := range AlertFieldCount {
			copy.Set(field, override.Value(field))
		}
		result[id] = copy
	}
	return result
}
