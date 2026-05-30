// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// EndpointSelector unit tests
// ---------------------------------------------------------------------------

func TestEndpointSelector_WeightedPrefersHealthy(t *testing.T) {
	hc := NewHealthCollector()
	// Seed health data: node1 healthy, node2 degraded via consecutive failures.
	hc.RecordRequest("http://node1.test", 50*time.Millisecond, true)
	hc.RecordRequest("http://node1.test", 40*time.Millisecond, true)
	for i := 0; i < 5; i++ {
		hc.RecordRequest("http://node2.test", 0, false)
	}

	policy := DefaultFailoverPolicy()
	sel := NewEndpointSelector(policy, hc)

	// Mark node2 degraded via consecutive failures through the selector.
	for i := 0; i < policy.MaxConsecutiveFailures; i++ {
		sel.RecordFailure("http://node2.test")
	}

	candidates := []string{"http://node1.test", "http://node2.test"}

	// Over many selections, node1 should dominate.
	node1Count := 0
	const iterations = 200
	for i := 0; i < iterations; i++ {
		chosen := sel.Select(candidates)
		if chosen == "http://node1.test" {
			node1Count++
		}
	}

	// node1 should win at least 80% of the time.
	assert.Greater(t, node1Count, iterations*80/100,
		"healthy node should be selected most of the time")
}

func TestEndpointSelector_StickyAlwaysPicksHealthiest(t *testing.T) {
	hc := NewHealthCollector()
	hc.RecordRequest("http://fast.test", 20*time.Millisecond, true)
	hc.RecordRequest("http://fast.test", 25*time.Millisecond, true)
	hc.RecordRequest("http://slow.test", 800*time.Millisecond, true)
	hc.RecordRequest("http://slow.test", 900*time.Millisecond, true)

	policy := FailoverPolicy{
		Strategy:               FailoverStrategySticky,
		DegradedThreshold:      0.3,
		RecoveryProbeInterval:  30 * time.Second,
		MaxConsecutiveFailures: 3,
	}
	sel := NewEndpointSelector(policy, hc)

	candidates := []string{"http://slow.test", "http://fast.test"}
	for i := 0; i < 20; i++ {
		chosen := sel.Select(candidates)
		assert.Equal(t, "http://fast.test", chosen,
			"sticky strategy should always pick the healthiest endpoint")
	}
}

func TestEndpointSelector_RoundRobinCycles(t *testing.T) {
	hc := NewHealthCollector()
	policy := FailoverPolicy{
		Strategy:               FailoverStrategyRoundRobin,
		DegradedThreshold:      0.3,
		RecoveryProbeInterval:  30 * time.Second,
		MaxConsecutiveFailures: 3,
	}
	sel := NewEndpointSelector(policy, hc)

	candidates := []string{"http://a.test", "http://b.test", "http://c.test"}
	seen := make(map[string]int)
	for i := 0; i < 9; i++ {
		chosen := sel.Select(candidates)
		seen[chosen]++
	}

	// Each node should be selected exactly 3 times in 9 iterations.
	for _, url := range candidates {
		assert.Equal(t, 3, seen[url], "round-robin should distribute evenly: %s", url)
	}
}

func TestEndpointSelector_RecordFailure_MarksDegraded(t *testing.T) {
	hc := NewHealthCollector()
	policy := DefaultFailoverPolicy()
	sel := NewEndpointSelector(policy, hc)

	url := "http://flaky.test"
	assert.False(t, sel.IsDegraded(url), "should not be degraded initially")

	for i := 0; i < policy.MaxConsecutiveFailures; i++ {
		sel.RecordFailure(url)
	}

	assert.True(t, sel.IsDegraded(url), "should be degraded after MaxConsecutiveFailures")
}

func TestEndpointSelector_RecordSuccess_ClearsDegraded(t *testing.T) {
	hc := NewHealthCollector()
	policy := DefaultFailoverPolicy()
	sel := NewEndpointSelector(policy, hc)

	url := "http://recovering.test"
	for i := 0; i < policy.MaxConsecutiveFailures; i++ {
		sel.RecordFailure(url)
	}
	require.True(t, sel.IsDegraded(url))

	sel.RecordSuccess(url)
	assert.False(t, sel.IsDegraded(url), "success should clear degraded state")
}

func TestEndpointSelector_DegradedURLs(t *testing.T) {
	hc := NewHealthCollector()
	policy := DefaultFailoverPolicy()
	sel := NewEndpointSelector(policy, hc)

	urls := []string{"http://a.test", "http://b.test", "http://c.test"}
	// Degrade a and c.
	for _, u := range []string{urls[0], urls[2]} {
		for i := 0; i < policy.MaxConsecutiveFailures; i++ {
			sel.RecordFailure(u)
		}
	}

	degraded := sel.DegradedURLs()
	assert.Len(t, degraded, 2)
	assert.Contains(t, degraded, urls[0])
	assert.Contains(t, degraded, urls[2])
	assert.NotContains(t, degraded, urls[1])
}

func TestEndpointSelector_FallsBackWhenAllDegraded(t *testing.T) {
	hc := NewHealthCollector()
	policy := FailoverPolicy{
		Strategy:               FailoverStrategySticky,
		DegradedThreshold:      0.3,
		RecoveryProbeInterval:  10 * time.Minute, // long window so no auto-recovery
		MaxConsecutiveFailures: 1,
	}
	sel := NewEndpointSelector(policy, hc)

	candidates := []string{"http://a.test", "http://b.test"}
	for _, u := range candidates {
		sel.RecordFailure(u)
	}

	// All degraded — selector must still return something.
	chosen := sel.Select(candidates)
	assert.NotEmpty(t, chosen, "selector must return a URL even when all are degraded")
	assert.Contains(t, candidates, chosen)
}

func TestEndpointSelector_ConcurrentSafety(t *testing.T) {
	hc := NewHealthCollector()
	policy := DefaultFailoverPolicy()
	sel := NewEndpointSelector(policy, hc)

	candidates := []string{
		"http://node1.test",
		"http://node2.test",
		"http://node3.test",
	}

	var wg sync.WaitGroup
	const goroutines = 50
	const ops = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				url := candidates[j%len(candidates)]
				switch j % 4 {
				case 0:
					_ = sel.Select(candidates)
				case 1:
					sel.RecordFailure(url)
				case 2:
					sel.RecordSuccess(url)
				case 3:
					_ = sel.IsDegraded(url)
				}
			}
		}(i)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: goroutines did not finish within 10 seconds")
	}
}

// ---------------------------------------------------------------------------
// Integration tests: adaptive failover through the RPC Client
// ---------------------------------------------------------------------------

func TestClient_SorobanAltURLs_FailoverToSecondEndpoint(t *testing.T) {
	// server1 always fails; server2 always succeeds.
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := GetHealthResponse{
			Jsonrpc: "2.0",
			ID:      1,
			Result: struct {
				Status                string `json:"status"`
				LatestLedger          uint32 `json:"latestLedger"`
				OldestLedger          uint32 `json:"oldestLedger"`
				LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
			}{Status: "healthy", LatestLedger: 42},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server2.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanAltURLs([]string{server1.URL, server2.URL}),
		WithAltURLs([]string{server1.URL, server2.URL}),
	)
	require.NoError(t, err)

	resp, err := client.GetHealth(context.Background())
	require.NoError(t, err, "should succeed after failing over to server2")
	assert.Equal(t, "healthy", resp.Result.Status)
}

func TestClient_SorobanAltURLs_StickySuccessSelection(t *testing.T) {
	// server1 is fast and healthy; server2 is slow.
	var server1Calls int32
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&server1Calls, 1)
		resp := GetHealthResponse{
			Jsonrpc: "2.0",
			ID:      1,
			Result: struct {
				Status                string `json:"status"`
				LatestLedger          uint32 `json:"latestLedger"`
				OldestLedger          uint32 `json:"oldestLedger"`
				LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
			}{Status: "healthy"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server1.Close()

	var server2Calls int32
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&server2Calls, 1)
		resp := GetHealthResponse{
			Jsonrpc: "2.0",
			ID:      1,
			Result: struct {
				Status                string `json:"status"`
				LatestLedger          uint32 `json:"latestLedger"`
				OldestLedger          uint32 `json:"oldestLedger"`
				LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
			}{Status: "healthy"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server2.Close()

	// Seed server1 as healthier in the collector before building the client.
	hc := NewHealthCollector()
	for i := 0; i < 5; i++ {
		hc.RecordRequest(server1.URL, 20*time.Millisecond, true)
	}
	for i := 0; i < 5; i++ {
		hc.RecordRequest(server2.URL, 900*time.Millisecond, true)
	}

	policy := FailoverPolicy{
		Strategy:               FailoverStrategySticky,
		DegradedThreshold:      0.3,
		RecoveryProbeInterval:  30 * time.Second,
		MaxConsecutiveFailures: 3,
	}

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanAltURLs([]string{server1.URL, server2.URL}),
		WithAltURLs([]string{server1.URL, server2.URL}),
		WithFailoverPolicy(policy),
	)
	require.NoError(t, err)

	// Replace the client's health collector with our pre-seeded one so the
	// selector sees the latency data we injected.
	client.healthCollector = hc
	client.selector = NewEndpointSelector(policy, hc)

	const requests = 10
	for i := 0; i < requests; i++ {
		_, err := client.GetHealth(context.Background())
		require.NoError(t, err)
	}

	// Sticky strategy should heavily favour server1 (healthier score).
	assert.Greater(t, atomic.LoadInt32(&server1Calls), int32(requests*7/10),
		"sticky strategy should route most traffic to the healthier endpoint")
}

func TestClient_SorobanAltURLs_RecoveryAfterTransientFailure(t *testing.T) {
	// server1 fails for the first 3 requests, then recovers.
	var callCount int32
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resp := GetHealthResponse{
			Jsonrpc: "2.0",
			ID:      1,
			Result: struct {
				Status                string `json:"status"`
				LatestLedger          uint32 `json:"latestLedger"`
				OldestLedger          uint32 `json:"oldestLedger"`
				LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
			}{Status: "healthy"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := GetHealthResponse{
			Jsonrpc: "2.0",
			ID:      1,
			Result: struct {
				Status                string `json:"status"`
				LatestLedger          uint32 `json:"latestLedger"`
				OldestLedger          uint32 `json:"oldestLedger"`
				LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
			}{Status: "healthy"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server2.Close()

	policy := FailoverPolicy{
		Strategy:               FailoverStrategyWeighted,
		DegradedThreshold:      0.3,
		RecoveryProbeInterval:  50 * time.Millisecond, // short for test
		MaxConsecutiveFailures: 2,
	}

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanAltURLs([]string{server1.URL, server2.URL}),
		WithAltURLs([]string{server1.URL, server2.URL}),
		WithFailoverPolicy(policy),
	)
	require.NoError(t, err)

	// First 3 requests: server1 fails, client should fall over to server2.
	for i := 0; i < 3; i++ {
		resp, err := client.GetHealth(context.Background())
		require.NoError(t, err, "request %d should succeed via server2", i+1)
		assert.Equal(t, "healthy", resp.Result.Status)
	}

	// server1 is now degraded. Wait for recovery probe interval.
	time.Sleep(100 * time.Millisecond)

	// After recovery interval, server1 should be eligible again.
	assert.False(t, client.selector.IsDegraded(server1.URL),
		"server1 should no longer be degraded after recovery interval")

	// Subsequent requests should succeed (server1 now healthy).
	resp, err := client.GetHealth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp.Result.Status)
}

func TestClient_SorobanAltURLs_AllNodesFailed_ReturnsAggregatedError(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server2.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanAltURLs([]string{server1.URL, server2.URL}),
		WithAltURLs([]string{server1.URL, server2.URL}),
	)
	require.NoError(t, err)

	_, err = client.GetHealth(context.Background())
	require.Error(t, err)

	var allFailed *AllNodesFailedError
	require.ErrorAs(t, err, &allFailed,
		"should return AllNodesFailedError when all Soroban endpoints fail")
	assert.Len(t, allFailed.Failures, 2,
		"should record one failure per endpoint")
}

func TestClient_WithFailoverPolicy_RoundRobin(t *testing.T) {
	var calls [3]int32
	servers := make([]*httptest.Server, 3)
	for i := range servers {
		idx := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&calls[idx], 1)
			resp := GetHealthResponse{
				Jsonrpc: "2.0",
				ID:      1,
				Result: struct {
					Status                string `json:"status"`
					LatestLedger          uint32 `json:"latestLedger"`
					OldestLedger          uint32 `json:"oldestLedger"`
					LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
				}{Status: "healthy"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer servers[i].Close()
	}

	urls := []string{servers[0].URL, servers[1].URL, servers[2].URL}
	policy := FailoverPolicy{
		Strategy:               FailoverStrategyRoundRobin,
		DegradedThreshold:      0.3,
		RecoveryProbeInterval:  30 * time.Second,
		MaxConsecutiveFailures: 5,
	}

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanAltURLs(urls),
		WithAltURLs(urls),
		WithFailoverPolicy(policy),
	)
	require.NoError(t, err)

	const total = 9
	for i := 0; i < total; i++ {
		_, err := client.GetHealth(context.Background())
		require.NoError(t, err)
	}

	// Each server should receive exactly 3 calls.
	for i, c := range calls {
		assert.Equal(t, int32(3), c,
			"round-robin: server %d should receive exactly 3 calls", i)
	}
}

func TestClient_HealthCollector_UpdatedUnderConcurrentLoad(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		resp := GetHealthResponse{
			Jsonrpc: "2.0",
			ID:      1,
			Result: struct {
				Status                string `json:"status"`
				LatestLedger          uint32 `json:"latestLedger"`
				OldestLedger          uint32 `json:"oldestLedger"`
				LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
			}{Status: "healthy"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(
		WithNetwork(Testnet),
		WithSorobanAltURLs([]string{server.URL}),
		WithAltURLs([]string{server.URL}),
	)
	require.NoError(t, err)

	const goroutines = 20
	const requestsPerGoroutine = 10
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				_, _ = client.GetHealth(context.Background())
			}
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("concurrent health check test timed out")
	}

	// Health collector should have recorded all requests.
	stats := client.healthCollector.GetStats(server.URL)
	require.NotNil(t, stats)
	assert.Equal(t, int64(goroutines*requestsPerGoroutine),
		stats.TotalRequests,
		"health collector should record all concurrent requests")
}
