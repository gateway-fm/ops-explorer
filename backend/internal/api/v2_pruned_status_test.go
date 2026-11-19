package api

import "testing"

func TestV2PrunedStatusPreservesOnlyFixedHistoryError(t *testing.T) {
	if !v2ReadStatus(410, []byte(`{"error":{"code":"history_pruned"}}`)) {
		t.Fatal("fixed pruning response became generic unavailability")
	}
	for _, raw := range []string{`{"error":{"code":"not_found"}}`, `{"error":{"code":"history_pruned","details":"private"}}`, `{"error":{"code":"history_pruned"},"extra":1}`} {
		if v2ReadStatus(410, []byte(raw)) {
			t.Fatal("unexpected pruning envelope passed the upstream boundary")
		}
	}
}
