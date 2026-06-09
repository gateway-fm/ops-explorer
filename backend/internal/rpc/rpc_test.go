package rpc

// Contract under test: internal/rpc.Client is a thin typed wrapper over a
// JSON-RPC 2.0 HTTP endpoint. The expected request methods/params and the
// response decoding below are derived from the ETHEREUM JSON-RPC SPECIFICATION
// (https://ethereum.org/en/developers/docs/apis/json-rpc/ and the
// execution-apis schema) — i.e. QUANTITY values are 0x-prefixed minimal hex,
// DATA values are 0x-prefixed byte hex, eth_getTransactionReceipt returns the
// documented receipt object, etc. — NOT by observing what the implementation
// happens to emit. Where the implementation would contradict the spec, the test
// is authoritative.
//
// These cases also pin the rpc-package-specific contract that does not live in
// pkg/eth/rpclient: authTransport injects `Authorization: Bearer <token>` from
// the request context (ContextKeyAuthToken), and error/ malformed-hex paths
// surface as Go errors.
//
// httptest handlers are defined inline; no package-level helpers are added that
// could collide with the privacy branch (`ph`) — local helpers are file-scoped
// closures and the one shared sentinel uses the `tc` prefix.

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"explorer/pkg/eth/common"
)

// tcRPCEnvelope is the JSON-RPC 2.0 request envelope the client is expected to
// send per the spec (jsonrpc:"2.0", numeric id, method, params array).
type tcRPCEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// tcNewClient spins an httptest server whose handler is `h`, returns an rpc
// Client pointed at it, and registers cleanup.
func tcNewClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL)
	if err != nil {
		t.Fatalf("rpc.New: %v", err)
	}
	return c
}

// tcRespondResult writes a successful JSON-RPC 2.0 response echoing the request
// id with the given raw result.
func tcRespondResult(t *testing.T, w http.ResponseWriter, reqID uint64, rawResult string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.WriteString(w, `{"jsonrpc":"2.0","id":`+itoa(reqID)+`,"result":`+rawResult+`}`); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

func itoa(n uint64) string { return new(big.Int).SetUint64(n).String() }

// tcDecodeReq decodes the inbound JSON-RPC request envelope.
func tcDecodeReq(t *testing.T, r *http.Request) tcRPCEnvelope {
	t.Helper()
	var env tcRPCEnvelope
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if env.JSONRPC != "2.0" {
		t.Errorf("request jsonrpc = %q, want \"2.0\" per spec", env.JSONRPC)
	}
	return env
}

func TestClientBlockNumber(t *testing.T) {
	// Spec: eth_blockNumber → QUANTITY. 0x4b7 == 1207.
	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		if env.Method != "eth_blockNumber" {
			t.Errorf("method = %q, want eth_blockNumber", env.Method)
		}
		tcRespondResult(t, w, env.ID, `"0x4b7"`)
	})

	got, err := c.BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	if got != 0x4b7 {
		t.Fatalf("BlockNumber = %d, want %d (0x4b7)", got, 0x4b7)
	}
}

func TestClientChainID(t *testing.T) {
	// Spec: eth_chainId → QUANTITY. 0x1 == mainnet.
	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		if env.Method != "eth_chainId" {
			t.Errorf("method = %q, want eth_chainId", env.Method)
		}
		tcRespondResult(t, w, env.ID, `"0x1"`)
	})

	got, err := c.ChainID(context.Background())
	if err != nil {
		t.Fatalf("ChainID: %v", err)
	}
	if got == nil || got.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("ChainID = %v, want 1", got)
	}
}

func TestClientGetBalance(t *testing.T) {
	// Spec: eth_getBalance(address, "latest") → QUANTITY (wei).
	// 0x0234c8a3397aab58 == 158972490234375000 wei.
	const wantWeiHex = "0x0234c8a3397aab58"
	wantWei, _ := new(big.Int).SetString("0234c8a3397aab58", 16)
	addr := common.HexToAddress("0x407d73d8a49eeb85d32cf465507dd71d507100c1")

	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		if env.Method != "eth_getBalance" {
			t.Errorf("method = %q, want eth_getBalance", env.Method)
		}
		// Per spec the second positional param is the block tag; default is "latest".
		var params []json.RawMessage
		if err := json.Unmarshal(env.Params, &params); err != nil {
			t.Fatalf("params: %v", err)
		}
		if len(params) != 2 {
			t.Fatalf("eth_getBalance params len = %d, want 2 (address, block)", len(params))
		}
		if string(params[1]) != `"latest"` {
			t.Errorf("block param = %s, want \"latest\"", params[1])
		}
		tcRespondResult(t, w, env.ID, `"`+wantWeiHex+`"`)
	})

	got, err := c.GetBalance(context.Background(), addr)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if got.Cmp(wantWei) != 0 {
		t.Fatalf("GetBalance = %s, want %s", got, wantWei)
	}
}

func TestClientGetCode(t *testing.T) {
	// Spec: eth_getCode → DATA (hex bytes). Assert raw bytes round-trip.
	const codeHex = "0x606060405260"
	addr := common.HexToAddress("0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b")

	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		if env.Method != "eth_getCode" {
			t.Errorf("method = %q, want eth_getCode", env.Method)
		}
		tcRespondResult(t, w, env.ID, `"`+codeHex+`"`)
	})

	got, err := c.GetCode(context.Background(), addr)
	if err != nil {
		t.Fatalf("GetCode: %v", err)
	}
	want := common.FromHex(codeHex)
	if string(got) != string(want) {
		t.Fatalf("GetCode = %x, want %x", got, want)
	}
}

func TestClientCallContract(t *testing.T) {
	// Spec: eth_call(callObject, block) → DATA. The call object must carry a
	// 0x-prefixed `data` field and (here) a `to`.
	to := common.HexToAddress("0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	input := common.FromHex("0x95d89b41") // symbol()
	const retHex = "0x0000000000000000000000000000000000000000000000000000000000000020"

	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		if env.Method != "eth_call" {
			t.Errorf("method = %q, want eth_call", env.Method)
		}
		var params []json.RawMessage
		if err := json.Unmarshal(env.Params, &params); err != nil {
			t.Fatalf("params: %v", err)
		}
		if len(params) != 2 {
			t.Fatalf("eth_call params len = %d, want 2 (callObj, block)", len(params))
		}
		var callObj map[string]string
		if err := json.Unmarshal(params[0], &callObj); err != nil {
			t.Fatalf("callObj: %v", err)
		}
		if callObj["to"] != to.Hex() {
			t.Errorf("call to = %q, want %q", callObj["to"], to.Hex())
		}
		if callObj["data"] != "0x95d89b41" {
			t.Errorf("call data = %q, want 0x95d89b41", callObj["data"])
		}
		tcRespondResult(t, w, env.ID, `"`+retHex+`"`)
	})

	got, err := c.CallContract(context.Background(), to, input)
	if err != nil {
		t.Fatalf("CallContract: %v", err)
	}
	if string(got) != string(common.FromHex(retHex)) {
		t.Fatalf("CallContract = %x, want %s", got, retHex)
	}
}

func TestClientTransactionByHash(t *testing.T) {
	// Spec: eth_getTransactionByHash → transaction object. A MINED tx has a
	// non-null blockNumber (so isPending == false); fields are 0x-quantities /
	// DATA. Values chosen from the spec's documented shapes.
	hash := common.HexToHash("0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b")
	const txObj = `{
		"hash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
		"blockNumber":"0x5daf3b",
		"from":"0xa7d9ddbe1f17865597fbd27ec712455208b6b76d",
		"to":"0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb",
		"value":"0xf3dbb76162000",
		"gas":"0xc350",
		"gasPrice":"0x4a817c800",
		"input":"0x68656c6c6f21",
		"nonce":"0x15",
		"type":"0x0"
	}`

	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		if env.Method != "eth_getTransactionByHash" {
			t.Errorf("method = %q, want eth_getTransactionByHash", env.Method)
		}
		tcRespondResult(t, w, env.ID, txObj)
	})

	tx, isPending, err := c.TransactionByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("TransactionByHash: %v", err)
	}
	if isPending {
		t.Fatalf("isPending = true, want false (blockNumber is set per spec ⇒ mined)")
	}
	if tx.Hash != hash {
		t.Errorf("hash = %s, want %s", tx.Hash.Hex(), hash.Hex())
	}
	if tx.BlockNumber == nil || tx.BlockNumber.ToInt().Uint64() != 0x5daf3b {
		t.Errorf("blockNumber = %v, want 0x5daf3b", tx.BlockNumber)
	}
	// gas 0xc350 == 50000
	if uint64(tx.Gas) != 0xc350 {
		t.Errorf("gas = %d, want 50000", uint64(tx.Gas))
	}
	if tx.To == nil || tx.To.Hex() != common.HexToAddress("0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb").Hex() {
		t.Errorf("to = %v, want 0xf02c...32bb", tx.To)
	}
	if string(tx.Input) != string(common.FromHex("0x68656c6c6f21")) {
		t.Errorf("input = %x, want 68656c6c6f21", []byte(tx.Input))
	}
}

func TestClientTransactionByHashPending(t *testing.T) {
	// Spec: a pending tx has blockNumber == null ⇒ isPending must be true.
	hash := common.HexToHash("0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b")
	const txObj = `{
		"hash":"0x88df016429689c079f3b2f6ad39fa052532c56795b733da78a91ebe6a713944b",
		"blockNumber":null,
		"from":"0xa7d9ddbe1f17865597fbd27ec712455208b6b76d",
		"to":"0xf02c1c8e6114b1dbe8937a39260b5b0a374432bb",
		"value":"0x0",
		"gas":"0xc350",
		"gasPrice":"0x4a817c800",
		"input":"0x",
		"nonce":"0x15",
		"type":"0x0"
	}`

	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		tcRespondResult(t, w, env.ID, txObj)
	})

	_, isPending, err := c.TransactionByHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("TransactionByHash: %v", err)
	}
	if !isPending {
		t.Fatalf("isPending = false, want true (null blockNumber per spec ⇒ pending)")
	}
}

func TestClientTransactionReceipt(t *testing.T) {
	// Spec: eth_getTransactionReceipt → receipt object. status 0x1 == success,
	// QUANTITY fields are hex. Values from the execution-apis receipt schema.
	hash := common.HexToHash("0xb903239f8543d04b5dc1ba6579132b143087c68db1b2168786408fcbce568238")
	const receiptObj = `{
		"transactionHash":"0xb903239f8543d04b5dc1ba6579132b143087c68db1b2168786408fcbce568238",
		"transactionIndex":"0x1",
		"blockNumber":"0xb",
		"status":"0x1",
		"gasUsed":"0x5208",
		"contractAddress":null,
		"logs":[]
	}`

	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		if env.Method != "eth_getTransactionReceipt" {
			t.Errorf("method = %q, want eth_getTransactionReceipt", env.Method)
		}
		tcRespondResult(t, w, env.ID, receiptObj)
	})

	rec, err := c.TransactionReceipt(context.Background(), hash)
	if err != nil {
		t.Fatalf("TransactionReceipt: %v", err)
	}
	if rec.Status != 1 {
		t.Errorf("status = %d, want 1 (0x1 ⇒ success per spec)", rec.Status)
	}
	if rec.GasUsed != 0x5208 { // 21000
		t.Errorf("gasUsed = %d, want 21000", rec.GasUsed)
	}
	if rec.TransactionIndex != 1 {
		t.Errorf("transactionIndex = %d, want 1", rec.TransactionIndex)
	}
	if rec.BlockNumber == nil || rec.BlockNumber.Uint64() != 0xb {
		t.Errorf("blockNumber = %v, want 0xb", rec.BlockNumber)
	}
	if rec.TxHash != hash {
		t.Errorf("txHash = %s, want %s", rec.TxHash.Hex(), hash.Hex())
	}
}

func TestClientRPCErrorObject(t *testing.T) {
	// Spec: a JSON-RPC error response carries an `error` object {code,message}
	// and no result. The client must surface this as a Go error (not silently
	// succeed). -32000 is a common server-error code.
	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+itoa(env.ID)+`,"error":{"code":-32000,"message":"execution reverted"}}`)
	})

	_, err := c.BlockNumber(context.Background())
	if err == nil {
		t.Fatalf("BlockNumber on JSON-RPC error response = nil error, want error surfaced")
	}
	if got := err.Error(); !containsAll(got, "-32000", "execution reverted") {
		t.Fatalf("error = %q, want it to carry code -32000 and the message", got)
	}
}

func TestClientMalformedHexResult(t *testing.T) {
	// Spec: QUANTITY must be 0x-prefixed hex. A non-hex string for a QUANTITY
	// result must fail to decode (hexutil rejects it) — not be silently coerced.
	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		env := tcDecodeReq(t, r)
		tcRespondResult(t, w, env.ID, `"notahexnumber"`)
	})

	_, err := c.BlockNumber(context.Background())
	if err == nil {
		t.Fatalf("BlockNumber with malformed hex result = nil error, want decode error")
	}
}

func TestAuthTransportInjectsBearer(t *testing.T) {
	// Contract specific to internal/rpc (not pkg/eth/rpclient): authTransport
	// sets `Authorization: Bearer <token>` when the request context carries a
	// non-empty ContextKeyAuthToken, and sets nothing otherwise.
	const token = "test-jwt-abc.def.ghi"

	var gotAuth string
	var sawHeader bool
	c := tcNewClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, sawHeader = r.Header["Authorization"]
		env := tcDecodeReq(t, r)
		tcRespondResult(t, w, env.ID, `"0x1"`)
	})

	// With a token in context → header injected.
	ctx := context.WithValue(context.Background(), ContextKeyAuthToken, token)
	if _, err := c.BlockNumber(ctx); err != nil {
		t.Fatalf("BlockNumber(with token): %v", err)
	}
	if gotAuth != "Bearer "+token {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer "+token)
	}

	// Without a token → no Authorization header at all.
	gotAuth, sawHeader = "", false
	if _, err := c.BlockNumber(context.Background()); err != nil {
		t.Fatalf("BlockNumber(no token): %v", err)
	}
	if sawHeader {
		t.Fatalf("Authorization header present (%q) with no token in context, want absent", gotAuth)
	}

	// With an empty-string token → still no header (the contract guards on != "").
	gotAuth, sawHeader = "", false
	ctxEmpty := context.WithValue(context.Background(), ContextKeyAuthToken, "")
	if _, err := c.BlockNumber(ctxEmpty); err != nil {
		t.Fatalf("BlockNumber(empty token): %v", err)
	}
	if sawHeader {
		t.Fatalf("Authorization header present (%q) with empty token, want absent", gotAuth)
	}
}

// containsAll reports whether s contains every substring in subs.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
