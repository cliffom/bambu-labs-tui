package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/cliffom/bambu-tui/internal/config"
	"github.com/cliffom/bambu-tui/internal/farm"
	"github.com/cliffom/bambu-tui/internal/notifications"
)

type (
	DecisionCommand struct {
		Target string
		Retry  bool
	}
	DecisionResult string

	webTarget struct {
		ID        string
		Condition farm.Condition
	}
	// BrowserPolicy carries current host policy and shutdown eligibility.
	BrowserPolicy struct {
		Alerts config.AlertPreferences
		Active bool
	}
)

// Publish sends independent display values and normalized notification values,
// never live farm pointers. Only the display DTO may reach HTTP clients.
func (m *Session) Publish(sink interface{ PublishDisplay(DisplayState) }, at time.Time, preferences ...BrowserPolicy) {
	if browser, ok := sink.(interface{ SubmitNotifications(notifications.Input) }); ok {
		policy := BrowserPolicy{Alerts: config.DefaultAlerts(), Active: true}
		if len(preferences) > 0 {
			policy = preferences[0]
		}
		input := m.BrowserNotificationInput(at, policy.Alerts)
		input.Active = policy.Active
		browser.SubmitNotifications(input)
	}
	sink.PublishDisplay(BuildDisplay(m.Farm, at, m.Demo, m.WebDecisions()))
}

func (m *Session) WebDecisions() Decisions {
	if m.webTargets == nil {
		m.webTargets = map[string]webTarget{}
	}
	for key, target := range m.webTargets {
		c, ok := m.Farm.Active[key]
		if !ok || !farm.SameOccurrence(key, target.Condition, c) {
			delete(m.webTargets, key)
		}
	}
	ids := map[string]string{}
	for key, c := range m.Farm.Active {
		target, ok := m.webTargets[key]
		if !ok {
			var id [16]byte
			if _, err := rand.Read(id[:]); err != nil {
				panic(err)
			}
			target = webTarget{ID: hex.EncodeToString(id[:]), Condition: c}
			m.webTargets[key] = target
		}
		ids[key] = target.ID
	}
	return Decisions{Targets: ids, State: m.ReviewSaveState()}
}

func (m *Session) ApplyDecision(c DecisionCommand, stopping, enabled bool) DecisionResult {
	result := DecisionStale
	if stopping || !enabled {
		return DecisionUnavailable
	}
	for key, target := range m.webTargets {
		current, ok := m.Farm.Active[key]
		if target.ID != c.Target || !ok || !farm.SameOccurrence(key, target.Condition, current) {
			continue
		}
		if c.Retry {
			if !current.Dismissed || !m.RetryPersistence(true, false) {
				break
			}
		} else if !current.Dismissed {
			m.Farm.Dismiss(key)
		}
		result = DecisionApplied
		break
	}
	return result
}

// BrowserNotificationInput shares normalized identities and alert policy with
// native delivery, while browser opt-in is independent from DesktopNotifications.
func (m *Session) BrowserNotificationInput(at time.Time, prefs config.AlertPreferences) notifications.Input {
	snapshot := m.Farm.NotificationSnapshot(at)
	allowed := map[string]bool{}
	for _, item := range m.Farm.Attention() {
		rules := prefs.RulesFor(item.PrinterID)
		rules.DesktopNotifications = true
		allowed[item.Key] = rules.AllowsNotification(item.Categories)
	}
	policies := map[string]notifications.Policy{}
	for id := range snapshot {
		rules := prefs.RulesFor(id)
		policies[id] = notifications.Policy{PrinterName: m.Farm.Printers[id].Name, Completion: rules.CompletedPrints, Sound: rules.NotificationSound, Snoozed: at.Before(m.Snoozes[id]), Allowed: allowed}
	}
	return notifications.Input{Snapshot: snapshot, Policies: policies, Active: true, Demo: m.Demo, Quiet: prefs.QuietHours.Active(at)}
}

const (
	DecisionApplied     DecisionResult = "applied"
	DecisionStale       DecisionResult = "Stale or unknown notice; refresh before retrying"
	DecisionUnavailable DecisionResult = "Session is stopping or web server disabled"
)
