package config

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // Schedules work even when the host has no zoneinfo database.
)

// QuietHours uses local wall-clock minutes and the weekday on which a period starts.
type QuietHours struct {
	Enabled  bool   `toml:"enabled"`
	Timezone string `toml:"timezone"`
	Start    string `toml:"start"`
	End      string `toml:"end"`
	Days     string `toml:"days"`
}

func clockMinute(value string) (int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil || len(value) != 5 {
		return 0, fmt.Errorf("quiet hours times must use HH:MM")
	}
	return t.Hour()*60 + t.Minute(), nil
}

func (q QuietHours) weekdays() (map[time.Weekday]bool, error) {
	days := map[time.Weekday]bool{}
	names := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	for _, name := range strings.Split(q.Days, ",") {
		found := false
		for day, match := range names {
			if strings.TrimSpace(name) == match {
				days[time.Weekday(day)] = true
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("quiet hours days must be comma-separated sun,mon,tue,wed,thu,fri,sat")
		}
	}
	return days, nil
}

func (q QuietHours) Validate() error {
	if !q.Enabled && q.Timezone == "" && q.Start == "" && q.End == "" && q.Days == "" {
		return nil
	}
	if q.Timezone == "" || q.Timezone == "Local" {
		return fmt.Errorf("quiet hours require an explicit IANA timezone")
	}
	if _, err := time.LoadLocation(q.Timezone); err != nil {
		return fmt.Errorf("quiet hours timezone: %w", err)
	}
	start, err := clockMinute(q.Start)
	if err != nil {
		return err
	}
	end, err := clockMinute(q.End)
	if err != nil {
		return err
	}
	if start == end {
		return fmt.Errorf("quiet hours start and end must differ")
	}
	_, err = q.weekdays()
	return err
}

// Active compares wall-clock minutes: skipped DST minutes never occur and
// repeated minutes have the same policy on both passes. Start inclusive, end exclusive.
func (q QuietHours) Active(now time.Time) bool {
	if !q.Enabled {
		return false
	}
	zone, err := time.LoadLocation(q.Timezone)
	if err != nil {
		return true
	}
	days, err := q.weekdays()
	if err != nil {
		return true
	}
	start, err := clockMinute(q.Start)
	if err != nil {
		return true
	}
	end, err := clockMinute(q.End)
	if err != nil {
		return true
	}
	local := now.In(zone)
	minute := local.Hour()*60 + local.Minute()
	day := local.Weekday()
	if start < end {
		return days[day] && minute >= start && minute < end
	}
	if minute >= start {
		return days[day]
	}
	return minute < end && days[(day+6)%7]
}
