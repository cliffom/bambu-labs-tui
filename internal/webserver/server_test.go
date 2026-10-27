package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cliffom/bambu-tui/internal/farm"
)

func TestReadOnlyRoutesAndBrowserBoundary(t *testing.T) {
	s := New(t.Context())
	port := availablePort(t)
	t.Cleanup(s.Close)
	status := s.SetEnabled(true, port)
	if status.Error != "" || !strings.HasPrefix(status.URL, "http://127.0.0.1:") {
		t.Fatal(status)
	}
	u, _ := url.Parse(status.URL)
	for _, test := range []struct {
		method, path, host, origin, site string
		code                             int
	}{
		{"GET", "/", u.Host, "", "", 200},
		{"GET", "/app.js", u.Host, "", "", 200},
		{"GET", "/style.css", u.Host, "", "", 200},
		{"GET", "/icon.png", u.Host, "", "", 200},
		{"GET", "/api/snapshot", u.Host, "http://" + u.Host, "same-origin", 200},
		{"HEAD", "/api/snapshot", u.Host, "", "", 200},
		{"POST", "/api/snapshot", u.Host, "", "", 405},
		{"PUT", "/api/snapshot", u.Host, "", "", 405},
		{"DELETE", "/", u.Host, "", "", 405},
		{"OPTIONS", "/api/snapshot", u.Host, "", "", 405},
		{"GET", "/api/snapshot", "attacker.example", "", "", 403},
		{"GET", "/api/snapshot", u.Host, "https://attacker.example", "", 403},
		{"GET", "/api/snapshot", u.Host, "", "cross-site", 403},
		{"GET", "/api/control", u.Host, "", "", 404},
		{"GET", "/index.html", u.Host, "", "", 404},
		{"GET", "/../server.go", u.Host, "", "", 404},
	} {
		req := httptest.NewRequest(test.method, "http://"+u.Host+test.path, nil)
		req.Host = test.host
		req.Header.Set("Origin", test.origin)
		req.Header.Set("Sec-Fetch-Site", test.site)
		w := httptest.NewRecorder()
		s.server.Handler.ServeHTTP(w, req)
		if w.Code != test.code {
			t.Fatalf("%s %s host=%s origin=%s: %d", test.method, test.path, test.host, test.origin, w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("missing browser boundary headers")
		}
		if test.method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD returned a body")
		}
	}
}

func TestEnableDisableCancellationAndLateCommands(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := New(ctx)
	port := availablePort(t)
	t.Cleanup(s.Close)
	if s.Status().URL != "" {
		t.Fatal("enabled by default")
	}
	first := s.SetEnabled(true, port)
	if got := s.SetEnabled(true, port); got != first {
		t.Fatal("enable created another listener")
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(first.URL + "/api/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if got := s.SetEnabled(false, port); got.URL != "" || got.Error != "" {
		t.Fatal(got)
	}
	if conn, err := net.DialTimeout("tcp", strings.TrimPrefix(first.URL, "http://"), time.Second); err == nil {
		_ = conn.Close()
		t.Fatal("disabled listener still open")
	}
	second := s.SetEnabled(true, port)
	if second.URL == "" {
		t.Fatal(second)
	}
	cancel()
	s.Close() // Joins cancellation cleanup without sleeps or an open port.
	if s.Status().URL != "" {
		t.Fatal("cancel left listener open")
	}
	if got := s.SetEnabled(true, port); got.URL != "" || got.Error == "" {
		t.Fatal("late command reopened canceled session")
	}
}

func TestClosePreventsLateStartEvenBeforeContextCancellation(t *testing.T) {
	s := New(t.Context())
	port := availablePort(t)
	s.Close()
	if got := s.SetEnabled(true, port); got.URL != "" || got.Error == "" {
		t.Fatal("late start after CLI cleanup", got)
	}
}

func TestConcurrentClientsSeeIndependentPublishedSnapshots(t *testing.T) {
	s := New(t.Context())
	port := availablePort(t)
	t.Cleanup(s.Close)
	data := []byte(`{"printers":[],"marker":"before"}`)
	s.Publish(data)
	copy(data, bytes.Repeat([]byte("x"), len(data)))
	status := s.SetEnabled(true, port)
	client := &http.Client{Timeout: 5 * time.Second}
	var wg sync.WaitGroup
	for range 24 {
		wg.Go(func() {
			for range 8 {
				response, err := client.Get(status.URL + "/api/snapshot")
				if err != nil {
					t.Error(err)
					return
				}
				body, err := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if err != nil || !json.Valid(body) || (!bytes.Contains(body, []byte("before")) && !bytes.Contains(body, []byte("after"))) {
					t.Errorf("torn snapshot: %q, %v", body, err)
					return
				}
			}
		})
	}
	for range 40 {
		s.Publish([]byte(`{"printers":[],"marker":"after"}`))
	}
	wg.Wait()
}

func syntheticState(t *testing.T) (*farm.State, time.Time) {
	t.Helper()
	now := time.Now().UTC()
	s := farm.New(10)
	s.Discover([]farm.Device{{ID: "PRIVATE-SERIAL", Name: "Synthetic <printer>", Model: "H2D", Online: true}}, now)
	s.Cloud(true, "PRIVATE-BACKEND-ERROR", now)
	err := s.Report("PRIVATE-SERIAL", []byte(`{"print":{"task_id":"PRIVATE-TASK","subtask_id":"PRIVATE-SUBTASK","gcode_state":"RUNNING","subtask_name":"Synthetic job","mc_percent":42,"mc_remaining_time":30,"bed_temper":60,"ams":{"ams":[{"id":0,"tray":[{"id":0,"tray_type":"PLA","remain":50,"tray_color":"AABBCCFF"}]}]},"access_token":"PRIVATE-TOKEN","account":"PRIVATE-ACCOUNT"}}`), now)
	if err != nil {
		t.Fatal(err)
	}
	return s, now
}

func decodeSnapshot(t *testing.T, data []byte) snapshot {
	t.Helper()
	var got snapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSnapshotAllowlistIsolationAndUnknowns(t *testing.T) {
	s, now := syntheticState(t)
	data := SnapshotJSON(s, now, true)
	for _, secret := range []string{"PRIVATE-SERIAL", "PRIVATE-TASK", "PRIVATE-SUBTASK", "PRIVATE-TOKEN", "PRIVATE-ACCOUNT", "PRIVATE-BACKEND-ERROR", "Raw", "access_token"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatal("sensitive field escaped allowlist", secret)
		}
	}
	got := decodeSnapshot(t, data)
	p := got.Printers[0]
	if !got.Demo || !p.Fresh || p.Progress == nil || *p.Progress != 42 || p.Chamber.Current != nil || p.AMS[0].Slots[0].Remaining == nil {
		t.Fatal("lost normalized or unknown values", p)
	}
	// Encoded bytes retain the prior complete observation, independent of later changes.
	*s.Printers["PRIVATE-SERIAL"].Progress = 90
	if *decodeSnapshot(t, data).Printers[0].Progress != 42 {
		t.Fatal("snapshot aliases mutable farm")
	}
}

func TestSnapshotReusesFreshnessETAAndAttention(t *testing.T) {
	s, now := syntheticState(t)
	p := s.Printers["PRIVATE-SERIAL"]
	if err := s.Report(p.ID, []byte(`{"print":{"gcode_state":"FAILED"}}`), now); err != nil {
		t.Fatal(err)
	}
	got := decodeSnapshot(t, SnapshotJSON(s, now, false))
	if got.Summary.Attention != 1 || !got.Printers[0].Attention || len(got.Printers[0].Conditions) == 0 {
		t.Fatal("failed-state attention missing")
	}
	for _, a := range s.Attention() {
		s.Acknowledge(a.Key, true)
	}
	got = decodeSnapshot(t, SnapshotJSON(s, now, false))
	if got.Summary.Attention != 0 || !got.Printers[0].Conditions[0].Reviewed {
		t.Fatal("local review lost")
	}
	if err := s.Report(p.ID, []byte(`{"print":{"gcode_state":"RUNNING","mc_remaining_time":30}}`), now); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		age           time.Duration
		online, cloud bool
		connection    string
		fresh, eta    bool
	}{
		{time.Second, true, true, farm.ConnectionLive, true, true},
		{3 * time.Minute, true, true, farm.ConnectionStale, false, false},
		{time.Second, false, true, farm.ConnectionOffline, false, false},
		{time.Second, true, false, farm.ConnectionCloudDisconnected, false, false},
		{time.Second, false, false, farm.ConnectionCloudDisconnected, false, false},
	} {
		p.Online = test.online
		s.Connected = test.cloud
		got = decodeSnapshot(t, SnapshotJSON(s, now.Add(test.age), false))
		if got.Printers[0].Online != test.online || got.Printers[0].Connection != test.connection || got.Printers[0].Fresh != test.fresh || got.Printers[0].ETACurrent != test.eta {
			t.Fatalf("freshness diverged: %+v", got.Printers[0])
		}
	}
	// A temperature-only report must not renew a remaining-time estimate.
	p.Online = true
	s.Connected = true
	if err := s.Report(p.ID, []byte(`{"print":{"bed_temper":61}}`), now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got = decodeSnapshot(t, SnapshotJSON(s, now.Add(3*time.Minute), false))
	if !got.Printers[0].Fresh || got.Printers[0].ETACurrent {
		t.Fatal("temperature refreshed ETA")
	}
	s.Discover([]farm.Device{{ID: "EMPTY", Name: "Waiting", Online: true}}, now)
	got = decodeSnapshot(t, SnapshotJSON(s, now, false))
	if got.Printers[0].Connection != farm.ConnectionWaiting || got.Printers[0].Progress != nil || got.Printers[0].Remaining != nil {
		t.Fatal("unknown startup inferred values")
	}
}

func TestSnapshotProgressAndETADoNotConfirmCompletion(t *testing.T) {
	state, now := syntheticState(t)
	if err := state.Report("PRIVATE-SERIAL", []byte(`{"print":{"mc_percent":100,"mc_remaining_time":0}}`), now); err != nil {
		t.Fatal(err)
	}
	got := decodeSnapshot(t, SnapshotJSON(state, now.Add(time.Minute), false))
	if got.Summary.Printing != 1 || got.Printers[0].State != farm.StateRunning {
		t.Fatal("progress or elapsed estimate implied completion")
	}
}

func availablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func TestConfiguredPortConflictFailsWithoutFallback(t *testing.T) {
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	port := occupied.Addr().(*net.TCPAddr).Port
	server := New(t.Context())
	t.Cleanup(server.Close)
	status := server.SetEnabled(true, port)
	if status.URL != "" || !strings.Contains(status.Error, "already in use") || server.server != nil {
		t.Fatal("occupied port started a listener or hid error", status)
	}
	if server.Status() != status {
		t.Fatal("conflict was not retained")
	}
	if err := occupied.Close(); err != nil {
		t.Fatal(err)
	}
	status = server.SetEnabled(true, port)
	if status.Error != "" || status.URL != fmt.Sprintf("http://127.0.0.1:%d", port) {
		t.Fatal("explicit retry did not bind the exact port", status)
	}
	for _, invalid := range []int{0, -1, 65536} {
		status = server.SetEnabled(true, invalid)
		if status.URL != "" || status.Error == "" || server.server != nil {
			t.Fatal("invalid port started server", invalid, status)
		}
	}
}

func TestChangingRunningPortStopsOldListenerAndHonorsConflict(t *testing.T) {
	server := New(t.Context())
	t.Cleanup(server.Close)
	firstPort, secondPort := availablePort(t), availablePort(t)
	first := server.SetEnabled(true, firstPort)
	// Reserve a target while the original listener remains active.
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	blockedPort := occupied.Addr().(*net.TCPAddr).Port
	status := server.SetEnabled(true, blockedPort)
	if status.URL != "" || !strings.Contains(status.Error, "already in use") || server.server != nil {
		t.Fatal("port change fell back to another listener", status)
	}
	if conn, err := net.DialTimeout("tcp", strings.TrimPrefix(first.URL, "http://"), time.Second); err == nil {
		_ = conn.Close()
		t.Fatal("old listener survived failed port change")
	}
	status = server.SetEnabled(true, secondPort)
	if status.Error != "" || status.URL != fmt.Sprintf("http://127.0.0.1:%d", secondPort) {
		t.Fatal(status)
	}
	response, err := (&http.Client{Timeout: time.Second}).Get(status.URL + "/api/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
}

func TestListenerUptimeFollowsSuccessfulLifetime(t *testing.T) {
	server := New(t.Context())
	defer server.Close()
	if !server.Status().Started.IsZero() {
		t.Fatal("unbound listener has uptime")
	}
	port := availablePort(t)
	first := server.SetEnabled(true, port)
	if first.Started.IsZero() || first.URL == "" {
		t.Fatal("bound listener lacks start", first)
	}
	if server.SetEnabled(true, port).Started != first.Started {
		t.Fatal("idempotent enable reset uptime")
	}
	stopped := server.SetEnabled(false, 0)
	if !stopped.Started.IsZero() || stopped.URL != "" {
		t.Fatal("stopped lifetime retained", stopped)
	}
	failed := server.SetEnabled(true, -1)
	if !failed.Started.IsZero() || failed.URL != "" || failed.Error == "" {
		t.Fatal("failed bind has uptime", failed)
	}
	second := server.SetEnabled(true, port)
	if !second.Started.After(first.Started) {
		t.Fatal("restart retained listener uptime")
	}
}
