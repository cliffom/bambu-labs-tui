package farm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const catalogueCommit = "79df7ae309be398ed30c9f397c13c9911c2edc57"

type upstreamCatalogue struct {
	Data map[string]struct {
		Entries []struct {
			Code string `json:"ecode"`
			Text string `json:"intro"`
		} `json:"en"`
	} `json:"data"`
}

// TestUpstreamFaultCatalogue audits bundled codes against immutable upstream
// catalogues. Network access is explicit; ordinary regression tests stay offline.
func TestUpstreamFaultCatalogue(t *testing.T) {
	if os.Getenv("BAMBU_AUDIT_UPSTREAM") != "1" {
		t.Skip("run make audit-faults to verify the pinned upstream catalogues")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second}
	known := map[string]map[string][]string{"device_hms": {}, "device_error": {}}
	for _, model := range []string{"093", "094", "20P", "22E", "239", "26A", "31B"} {
		catalogue := fetchCatalogue(t, ctx, client, model)
		for family, codes := range known {
			entries := catalogue.Data[family].Entries
			if len(entries) == 0 {
				t.Fatalf("catalogue %s has no %s entries", model, family)
			}
			for _, entry := range entries {
				codes[strings.ToUpper(entry.Code)] = append(codes[strings.ToUpper(entry.Code)], entry.Text)
			}
		}
	}
	for _, family := range []struct {
		name     string
		messages map[string]string
	}{{"device_hms", hmsMessages}, {"device_error", printErrors}} {
		keys := make([]string, 0, len(family.messages))
		for key := range family.messages {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			code := strings.ReplaceAll(key, "_", "")
			texts, exists := known[family.name][code]
			if !exists {
				t.Errorf("%s %s is absent from pinned upstream catalogues", family.name, key)
				continue
			}
			if family.name == "device_hms" {
				auditSlotLabel(t, key, family.messages[key], texts)
			}
		}
		t.Logf("Audited %d %s mappings against Bambu Studio %s", len(keys), family.name, catalogueCommit)
	}
}

// fetchCatalogue reads one English catalogue from the pinned Bambu Studio commit.
func fetchCatalogue(t *testing.T, ctx context.Context, client *http.Client, model string) upstreamCatalogue {
	t.Helper()
	url := fmt.Sprintf("https://raw.githubusercontent.com/bambulab/BambuStudio/%s/resources/hms/hms_en_%s.json", catalogueCommit, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("fetch catalogue %s: %v", model, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetch catalogue %s: HTTP %d", model, resp.StatusCode)
	}
	var catalogue upstreamCatalogue
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&catalogue); err != nil {
		t.Fatalf("decode catalogue %s: %v", model, err)
	}
	return catalogue
}

var localSlotLabel = regexp.MustCompile(`^AMS(?: HT)? \d+ slot (\d+):`)
var upstreamSlotLabel = regexp.MustCompile(`(?i)\bslot\s*(\d+)`)

// auditSlotLabel checks that generated slot labels agree with upstream messages.
func auditSlotLabel(t *testing.T, code, text string, upstream []string) {
	t.Helper()
	local := localSlotLabel.FindStringSubmatch(text)
	if local == nil {
		return
	}
	want, err := strconv.Atoi(local[1])
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, source := range upstream {
		for _, slot := range upstreamSlotLabel.FindAllStringSubmatch(source, -1) {
			got, err := strconv.Atoi(slot[1])
			if err != nil {
				t.Fatal(err)
			}
			matched = true
			if got != want {
				t.Errorf("%s labels slot %d, but upstream labels slot %d", code, want, got)
			}
		}
	}
	if !matched {
		t.Errorf("%s has a local slot label but no upstream slot evidence", code)
	}
}
