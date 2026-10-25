package mqtt

import "testing"

func TestReportPrinterID(t *testing.T) {
	for _, tc := range []struct {
		topic, want string
		ok          bool
	}{
		{topic: "device/TEST/report", want: "TEST", ok: true},
		{topic: "device/A_B-1/report", want: "A_B-1", ok: true},
		// Topic shape is checked here; discovered-printer identity is checked upstream.
		{topic: "device//report", ok: true},
		{topic: "other/TEST/report"},
		{topic: "device/TEST/request"},
		{topic: "device/TEST/report/extra"},
		{topic: "device/TEST/extra/report"},
		{topic: "device/TEST"},
		{topic: "/device/TEST/report"},
	} {
		t.Run(tc.topic, func(t *testing.T) {
			got, ok := reportPrinterID(tc.topic)
			if ok != tc.ok || (ok && got != tc.want) {
				t.Fatalf("reportPrinterID(%q) = %q, %v; want %q, %v", tc.topic, got, ok, tc.want, tc.ok)
			}
		})
	}
}
