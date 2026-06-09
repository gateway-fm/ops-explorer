package publicapi

// BUG (audit §3 publicapi): handleGetSyncStatus computes
//   blocksRemaining = int64(latestOnChain) - int64(status.LastIndexedBlock)
// When the indexer is AHEAD of the chain-height read (a normal transient race:
// the indexer observed a block newer than this GetLatestBlockNumber call), the
// subtraction goes NEGATIVE. "Blocks remaining" can never be negative — it must
// floor at 0 (and isSynced is already true in that case). A negative count
// confuses any client/progress bar consuming it.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"explorer/internal/types"
)

type tcSyncProvider struct {
	tcProvider
	syncStatus *types.SyncStatus
	latest     uint64
}

func (p *tcSyncProvider) GetSyncStatus(context.Context) (*types.SyncStatus, error) {
	return p.syncStatus, nil
}
func (p *tcSyncProvider) GetLatestBlockNumber(context.Context) (uint64, error) {
	return p.latest, nil
}

func TestHandleGetSyncStatus_IndexerAhead_NoNegativeRemaining(t *testing.T) {
	// Indexer at 105, chain read at 100 (indexer ahead by 5).
	prov := &tcSyncProvider{
		syncStatus: &types.SyncStatus{LastIndexedBlock: 105, UpdatedAt: time.Unix(0, 0)},
		latest:     100,
	}
	s := tcServer(prov)
	w := tcReq(s, "/api/v1/sync", "203.0.113.70:1")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var resp struct {
		BlocksRemaining int64 `json:"blocksRemaining"`
		IsSynced        bool  `json:"isSynced"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.BlocksRemaining < 0 {
		t.Errorf("blocksRemaining = %d, must never be negative when the indexer is ahead", resp.BlocksRemaining)
	}
	if resp.BlocksRemaining != 0 {
		t.Errorf("blocksRemaining = %d, want 0 (indexer ahead => synced)", resp.BlocksRemaining)
	}
	if !resp.IsSynced {
		t.Errorf("isSynced = false, want true when LastIndexedBlock >= latestChainBlock")
	}
}

func TestHandleGetSyncStatus_NormalBehind(t *testing.T) {
	prov := &tcSyncProvider{
		syncStatus: &types.SyncStatus{LastIndexedBlock: 90, UpdatedAt: time.Unix(0, 0)},
		latest:     100,
	}
	s := tcServer(prov)
	w := tcReq(s, "/api/v1/sync", "203.0.113.71:1")
	var resp struct {
		BlocksRemaining int64 `json:"blocksRemaining"`
		IsSynced        bool  `json:"isSynced"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.BlocksRemaining != 10 {
		t.Errorf("blocksRemaining = %d, want 10", resp.BlocksRemaining)
	}
	if resp.IsSynced {
		t.Errorf("isSynced = true, want false (10 behind)")
	}
}
