package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNotificationDoctorReportsAcceptanceWithoutClaimingDelivery(t *testing.T) {
	for _, sound := range []bool{false, true} {
		var out bytes.Buffer
		calls := 0
		err := notificationDoctor(context.Background(), &out, sound, func(_ context.Context, got bool) error {
			calls++
			if got != sound {
				t.Fatal("doctor changed sound request")
			}
			return nil
		})
		if err != nil || calls != 1 || !strings.Contains(out.String(), "delivery are not confirmed") {
			t.Fatalf("unexpected doctor result: %v, %s", err, &out)
		}
		if !strings.Contains(out.String(), "branded Bambu Stand helper") || !strings.Contains(out.String(), "Focus settings") || strings.Contains(out.String(), "Script Editor") {
			t.Fatalf("doctor must describe the branded helper and its suppression settings: %s", &out)
		}
	}
}

func TestNotificationDoctorFailureDoesNotReportAcceptance(t *testing.T) {
	var out bytes.Buffer
	failure := errors.New("delivery unavailable")
	err := notificationDoctor(context.Background(), &out, false, func(context.Context, bool) error { return failure })
	if !errors.Is(err, failure) || strings.Contains(out.String(), "accepted") {
		t.Fatalf("failure reported as acceptance: %v, %s", err, &out)
	}
}

func TestNotificationFlagsRequireDoctor(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--notify"}, "doctor options"},
		{[]string{"login", "--sound"}, "doctor options"},
		{[]string{"doctor", "--sound"}, "requires doctor --notify"},
	} {
		if err := run(tc.args); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%v: expected %q, got %v", tc.args, tc.want, err)
		}
	}
}

func TestNotificationDoctorBypassesConfigurationAndCredentials(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "synthetic-ssh-session")
	err := run([]string{"doctor", "--notify", "--config", "/nonexistent/bambustand-test/config.toml"})
	if err == nil || (!strings.Contains(err.Error(), "SSH is unsupported") && !strings.Contains(err.Error(), "macOS only")) {
		t.Fatalf("notification test should reach backend without configuration or credentials: %v", err)
	}
}
