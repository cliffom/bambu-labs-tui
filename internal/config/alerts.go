package config

import "github.com/cliffom/bambu-tui/internal/farm"

type (
	// AlertRules controls automatic modals without filtering underlying notices.
	AlertRules struct {
		AutomaticModals bool `toml:"automatic_modals"`
		Filament        bool `toml:"filament"`
		Extrusion       bool `toml:"extrusion"`
		PrinterFaults   bool `toml:"printer_faults"`
		FailedPrints    bool `toml:"failed_prints"`
		UnknownPauses   bool `toml:"unknown_pauses"`
	}

	// AlertOverride inherits each unspecified field from the global defaults.
	AlertOverride struct {
		AutomaticModals *bool `toml:"automatic_modals"`
		Filament        *bool `toml:"filament"`
		Extrusion       *bool `toml:"extrusion"`
		PrinterFaults   *bool `toml:"printer_faults"`
		FailedPrints    *bool `toml:"failed_prints"`
		UnknownPauses   *bool `toml:"unknown_pauses"`
	}

	// AlertPreferences contains non-secret global defaults and stable printer-ID overrides.
	AlertPreferences struct {
		AlertRules
		Printers map[string]AlertOverride `toml:"printers"`
	}
)

// DefaultAlerts preserves the existing automatic intervention-alert behavior.
func DefaultAlerts() AlertPreferences {
	return AlertPreferences{AlertRules: AlertRules{AutomaticModals: true, Filament: true, Extrusion: true, PrinterFaults: true, FailedPrints: true, UnknownPauses: true}}
}

// RulesFor resolves a printer's overrides independently for every setting.
func (p AlertPreferences) RulesFor(id string) AlertRules {
	r := p.AlertRules
	o := p.Printers[id]
	apply := func(dst *bool, value *bool) {
		if value != nil {
			*dst = *value
		}
	}
	apply(&r.AutomaticModals, o.AutomaticModals)
	apply(&r.Filament, o.Filament)
	apply(&r.Extrusion, o.Extrusion)
	apply(&r.PrinterFaults, o.PrinterFaults)
	apply(&r.FailedPrints, o.FailedPrints)
	apply(&r.UnknownPauses, o.UnknownPauses)
	return r
}

// Allows reports whether an eligible occurrence has an enabled alert category.
func (r AlertRules) Allows(categories []string) bool {
	if !r.AutomaticModals {
		return false
	}
	for _, category := range categories {
		switch category {
		case farm.AlertFilament:
			if r.Filament {
				return true
			}
		case farm.AlertExtrusion:
			if r.Extrusion {
				return true
			}
		case farm.AlertPrinterFaults:
			if r.PrinterFaults {
				return true
			}
		case farm.AlertFailedPrints:
			if r.FailedPrints {
				return true
			}
		case farm.AlertUnknownPauses:
			if r.UnknownPauses {
				return true
			}
		}
	}
	return false
}

// AnyCategoryEnabled reports whether automatic modals can interrupt for any supported category.
func (r AlertRules) AnyCategoryEnabled() bool {
	return r.Allows([]string{farm.AlertFilament, farm.AlertExtrusion, farm.AlertPrinterFaults, farm.AlertFailedPrints, farm.AlertUnknownPauses})
}
