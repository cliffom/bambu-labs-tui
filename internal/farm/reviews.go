package farm

import (
	"fmt"
	"strings"
	"time"
)

// Review contains only local notice decisions, never credentials or telemetry.
// Text identifies cloud/print-state changes; changing filament percentages and
// printer names do not change the identity of an ongoing notice.
type Review struct {
	Level        string    `json:"level"`
	Code         string    `json:"code,omitempty"`
	Text         string    `json:"text,omitempty"`
	Since        time.Time `json:"since"`
	Acknowledged bool      `json:"acknowledged,omitempty"`
	Dismissed    bool      `json:"dismissed,omitempty"`
}

func reviewText(key string, c Condition) string {
	if key == "cloud" || strings.HasSuffix(key, "/state") {
		return c.Text
	}
	return ""
}

func (r Review) matches(key string, c Condition) bool {
	return r.Level == c.Level && r.Code == c.Code && r.Text == reviewText(key, c)
}

func (s *State) rememberReview(key string, c Condition) {
	if !c.Acknowledged && !c.Dismissed {
		s.forgetReview(key)
		return
	}
	if s.reviews == nil {
		s.reviews = map[string]Review{}
	}
	r := Review{Level: c.Level, Code: c.Code, Text: reviewText(key, c), Since: c.Since, Acknowledged: c.Acknowledged, Dismissed: c.Dismissed}
	if old, ok := s.reviews[key]; !ok || old != r {
		s.reviews[key] = r
		s.reviewRevision++
	}
}

func (s *State) forgetReview(key string) {
	if _, ok := s.reviews[key]; ok {
		delete(s.reviews, key)
		s.reviewRevision++
	}
}

// RestoreReviews queues saved decisions without creating synthetic conditions.
// Call before processing discovery or telemetry.
func (s *State) RestoreReviews(reviews map[string]Review) {
	s.reviews = make(map[string]Review, len(reviews))
	for k, r := range reviews {
		if r.Acknowledged || r.Dismissed {
			s.reviews[k] = r
		}
	}
}

func (s *State) Reviews() map[string]Review {
	reviews := make(map[string]Review, len(s.reviews))
	for k, r := range s.reviews {
		reviews[k] = r
	}
	return reviews
}

func (s *State) ReviewRevision() uint64 { return s.reviewRevision }

// reviewKnown distinguishes a confirmed absence from data not received yet.
func (s *State) reviewKnown(key string) bool {
	if key == "cloud" {
		return s.cloudObserved
	}
	id, family, _ := strings.Cut(key, "/")
	p := s.Printers[id]
	if p == nil {
		return s.discovered
	}
	switch {
	case family == "offline":
		return true // Discovery supplies online status.
	case family == "stale":
		return !p.LastReport.IsZero()
	case family == "state":
		_, ok := p.Raw["gcode_state"]
		return ok
	case strings.HasPrefix(family, "fault/"):
		field := "print_error"
		if strings.Count(strings.TrimPrefix(family, "fault/"), "_") == 3 {
			field = "hms"
		}
		_, ok := p.Raw[field]
		return ok
	case strings.HasPrefix(family, "low/"):
		var amsID, slotID int
		if _, err := fmt.Sscanf(family, "low/%d/%d", &amsID, &slotID); err != nil {
			return true
		}
		units, ok := object(p.Raw["ams"])["ams"].([]any)
		if !ok {
			return false
		}
		if len(units) == 0 {
			return true
		}
		for _, unit := range units {
			u := object(unit)
			if n := number(u["id"]); n == nil || *n != amsID {
				continue
			}
			trays, ok := u["tray"].([]any)
			if ok && len(trays) == 0 {
				return true
			}
			for _, tray := range trays {
				v := object(tray)
				if n := number(v["id"]); n != nil && *n == slotID {
					_, remaining := v["remain"]
					state := number(v["state"])
					return remaining || (state != nil && *state&1 == 0) || str(v["tray_type"]) == "Empty"
				}
			}
		}
	}
	return false
}
