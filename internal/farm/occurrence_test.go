package farm

import (
	"testing"
	"time"
)

func TestLocalOccurrenceComparisonKeepsDistinctLifetimeRules(t *testing.T) {
	at := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	for _, key := range []string{ConditionCloud, "SYNTHETIC/" + ConditionState, "SYNTHETIC/" + ConditionLowPrefix + "0/0"} {
		a := Condition{Level: LevelWarning, Text: "first", Code: "code", Since: at}
		b := a
		b.Printer = "renamed"
		b.Acknowledged = true
		b.Dismissed = true
		if !SameOccurrence(key, a, b) {
			t.Fatal("review/name changed occurrence", key)
		}
		b.Since = at.Add(time.Second)
		if SameOccurrence(key, a, b) || !SameCondition(key, a, b) {
			t.Fatal("timestamp conflated semantic/lifetime identity", key)
		}
		b = a
		b.Text = "changed"
		want := key != ConditionCloud && key != "SYNTHETIC/"+ConditionState
		if SameCondition(key, a, b) != want {
			t.Fatal("text identity drift", key)
		}
		r := Review{Level: a.Level, Code: a.Code, Text: reviewText(key, a)}
		if r.matches(key, b) != want {
			t.Fatal("restored review differs from reconciliation", key)
		}
		b = a
		b.Level = LevelError
		if SameCondition(key, a, b) {
			t.Fatal("severity ignored")
		}
		b = a
		b.Code = "new"
		if SameCondition(key, a, b) {
			t.Fatal("code ignored")
		}
	}
	if (Review{Level: LevelWarning, Text: "unexpected saved text"}).matches("SYNTHETIC/low/0/0", Condition{Level: LevelWarning}) {
		t.Fatal("noncanonical saved review accepted")
	}
}
