package config

// ViewChoice stores stable names rather than UI enum positions. Unknown or
// non-string values from another version normalize independently to defaults.
type (
	ViewChoice      string
	ViewPreferences struct {
		Filter ViewChoice `toml:"filter"`
		Sort   ViewChoice `toml:"sort"`
	}
)

const (
	FilterAll         ViewChoice = "all"
	FilterAttention   ViewChoice = "attention"
	FilterPrinting    ViewChoice = "printing"
	FilterUnavailable ViewChoice = "unavailable"
	SortName          ViewChoice = "name"
	SortAttention     ViewChoice = "attention"
	SortState         ViewChoice = "state"
	SortFinish        ViewChoice = "finish"
)

func (v *ViewChoice) UnmarshalTOML(value any) error {
	text, _ := value.(string)
	*v = ViewChoice(text)
	return nil
}

func (v ViewPreferences) Normalized() ViewPreferences {
	switch v.Filter {
	case FilterAll, FilterAttention, FilterPrinting, FilterUnavailable:
	default:
		v.Filter = FilterAll
	}
	switch v.Sort {
	case SortName, SortAttention, SortState, SortFinish:
	default:
		v.Sort = SortName
	}
	return v
}
