// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/dotandev/glassbox/internal/logger"
)

// FailoverStrategy controls how the client selects among multiple Soroban RPC endpoints.
type FailoverStrategy int

const (
	// FailoverStrategyWeighted selects endpoints proportionally to their health score.
	// Healthy endpoints receive more traffic; degraded ones are avoided but not excluded.
	FailoverStrategyWeighted FailoverStrategy = iota

	// FailoverStrategySticky always uses the healthiest endpoint and only switches
	// when the current one is marked degraded.
	FailoverStrategySticky

	// FailoverStrategyRoundRobin cycles through all healthy endpoints in order.
	FailoverStrategyRoundRobin
)

// FailoverPolicy configures the adaptive endpoint selection behaviour.
type FailoverPolicy struct {
	// Strategy determines how the next endpoint is chosen on each request.
	Strategy FailoverStrategy

	// DegradedThreshold is the minimum health score below which an endpoint is
	// considered degraded and deprioritised. Range [0, 1]. Default: 0.3.
	DegradedThreshold float64

	// RecoveryProbeInterval is how long to wait before probing a degraded endpoint
	// again. Default: 30 seconds.
	RecoveryProbeInterval time.Duration

	// MaxConsecutiveFailures is the number of consecutive failures before an
	// endpoint is marked degraded regardless of its health score. Default: 3.
	MaxConsecutiveFailures int
}

// DefaultFailoverPolicy returns a sensible production-ready policy.
func DefaultFailoverPolicy() FailoverPolicy {
	return FailoverPolicy{
		Strategy:               FailoverStrategyWeighted,
		DegradedThreshold:      0.3,
		RecoveryProbeInterval:  30 * time.Second,
		MaxConsecutiveFailures: 3,
	}
}

// endpointState tracks per-endpoint runtime state used by the failover selector.
type endpointState struct {
	url                string
	consecutiveFailures int
	degradedSince      time.Time // zero if not degraded
	lastProbe          time.Time
}

// isDegraded reports whether this endpoint is currently in the degraded state.
func (s *endpointState) isDegraded(policy FailoverPolicy) bool {
	if s.degradedSince.IsZero() {
		return false
	}
	// Allow re-probe after the recovery interval.
	if time.Since(s.lastProbe) >= policy.RecoveryProbeInterval {
		return false
	}
	return true
}

// EndpointSelector implements adaptive, concurrency-safe endpoint selection.
type EndpointSelector struct {
	mu        sync.Mutex
	policy    FailoverPolicy
	states    map[string]*endpointState
	rrIndex   int // round-robin cursor
	collector *HealthCollector
}

// NewEndpointSelector creates a selector for the given policy and health collector.
func NewEndpointSelector(policy FailoverPolicy, collector *HealthCollector) *EndpointSelector {
	return &EndpointSelector{
		policy:    policy,
		states:    make(map[string]*endpointState),
		collector: collector,
	}
}

// RecordSuccess marks a successful request to url and resets its failure counter.
func (s *EndpointSelector) RecordSuccess(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.getOrCreate(url)
	st.consecutiveFailures = 0
	st.degradedSince = time.Time{} // clear degraded state
}

// RecordFailure records a failure for url and may mark it degraded.
func (s *EndpointSelector) RecordFailure(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.getOrCreate(url)
	st.consecutiveFailures++
	if st.consecutiveFailures >= s.policy.MaxConsecutiveFailures && st.degradedSince.IsZero() {
		st.degradedSince = time.Now()
		st.lastProbe = time.Now()
		logger.Logger.Warn("Endpoint marked degraded after consecutive failures",
			"url", url,
			"consecutive_failures", st.consecutiveFailures,
		)
	}
}

// Select picks the best endpoint from candidates according to the policy.
// It never returns an empty string if candidates is non-empty.
func (s *EndpointSelector) Select(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.policy.Strategy {
	case FailoverStrategySticky:
		return s.selectSticky(candidates)
	case FailoverStrategyRoundRobin:
		return s.selectRoundRobin(candidates)
	default: // FailoverStrategyWeighted
		return s.selectWeighted(candidates)
	}
}

// selectSticky returns the healthiest non-degraded endpoint, falling back to
// the least-degraded one if all are degraded.
func (s *EndpointSelector) selectSticky(candidates []string) string {
	healthy := s.filterHealthy(candidates)
	if len(healthy) > 0 {
		return s.highestScore(healthy)
	}
	// All degraded – pick the one whose probe interval has elapsed (allow recovery).
	for _, url := range candidates {
		st := s.getOrCreate(url)
		if time.Since(st.lastProbe) >= s.policy.RecoveryProbeInterval {
			st.lastProbe = time.Now()
			return url
		}
	}
	// Last resort: return the first candidate.
	return candidates[0]
}

// selectWeighted performs weighted random selection proportional to health scores.
func (s *EndpointSelector) selectWeighted(candidates []string) string {
	type scored struct {
		url    string
		weight float64
	}

	items := make([]scored, 0, len(candidates))
	totalWeight := 0.0

	for _, url := range candidates {
		score := s.scoreFor(url)
		st := s.getOrCreate(url)
		if st.isDegraded(s.policy) {
			// Degraded endpoints get a tiny weight so they can still be probed.
			score = 0.01
		}
		items = append(items, scored{url: url, weight: score})
		totalWeight += score
	}

	if totalWeight <= 0 {
		return candidates[0]
	}

	r := rand.Float64() * totalWeight
	cumulative := 0.0
	for _, item := range items {
		cumulative += item.weight
		if r <= cumulative {
			return item.url
		}
	}
	return items[len(items)-1].url
}

// selectRoundRobin cycles through healthy endpoints in order.
func (s *EndpointSelector) selectRoundRobin(candidates []string) string {
	healthy := s.filterHealthy(candidates)
	if len(healthy) == 0 {
		healthy = candidates // fall back to all
	}
	s.rrIndex = s.rrIndex % len(healthy)
	chosen := healthy[s.rrIndex]
	s.rrIndex = (s.rrIndex + 1) % len(healthy)
	return chosen
}

// filterHealthy returns candidates that are not currently degraded.
func (s *EndpointSelector) filterHealthy(candidates []string) []string {
	out := make([]string, 0, len(candidates))
	for _, url := range candidates {
		st := s.getOrCreate(url)
		if !st.isDegraded(s.policy) {
			out = append(out, url)
		}
	}
	return out
}

// highestScore returns the URL with the highest health score from the list.
func (s *EndpointSelector) highestScore(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	type us struct {
		url   string
		score float64
	}
	scored := make([]us, len(urls))
	for i, url := range urls {
		scored[i] = us{url: url, score: s.scoreFor(url)}
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})
	return scored[0].url
}

// scoreFor returns the health score for a URL from the collector, or 0.5 if unknown.
func (s *EndpointSelector) scoreFor(url string) float64 {
	if s.collector == nil {
		return 0.5
	}
	stats := s.collector.GetStats(url)
	if stats == nil {
		return 0.5 // neutral for unseen endpoints
	}
	return stats.HealthScore
}

// getOrCreate returns the endpointState for url, creating it if absent.
// Caller must hold s.mu.
func (s *EndpointSelector) getOrCreate(url string) *endpointState {
	st, ok := s.states[url]
	if !ok {
		st = &endpointState{url: url}
		s.states[url] = st
	}
	return st
}

// IsDegraded reports whether url is currently in the degraded state.
// Safe for concurrent use.
func (s *EndpointSelector) IsDegraded(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.getOrCreate(url)
	return st.isDegraded(s.policy)
}

// DegradedURLs returns a snapshot of all currently degraded endpoint URLs.
func (s *EndpointSelector) DegradedURLs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for url, st := range s.states {
		if st.isDegraded(s.policy) {
			out = append(out, url)
		}
	}
	return out
}
