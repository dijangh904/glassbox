// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

// Package perfmetrics provides lightweight timing metrics for debug sessions.
// It captures RPC call durations and simulator execution time, then renders a
// human-readable summary that is printed at the end of a debug session when
// the --show-metrics flag is set.
package perfmetrics

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// RPCRecord holds timing data for a single RPC call.
type RPCRecord struct {
	Method   string
	Duration time.Duration
	Err      bool
}

// Collector accumulates timing metrics during a debug session.
type Collector struct {
	mu          sync.Mutex
	rpcRecords  []RPCRecord
	simStart    time.Time
	simDuration time.Duration
	simRecorded bool
}

// NewCollector creates a ready-to-use Collector.
func NewCollector() *Collector {
	return &Collector{}
}

// RecordRPC records the duration of a single RPC call.
func (c *Collector) RecordRPC(method string, d time.Duration, err bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rpcRecords = append(c.rpcRecords, RPCRecord{Method: method, Duration: d, Err: err})
}

// StartSim marks the start of simulator execution.
func (c *Collector) StartSim() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.simStart = time.Now()
}

// StopSim marks the end of simulator execution.
func (c *Collector) StopSim() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.simStart.IsZero() {
		c.simDuration = time.Since(c.simStart)
		c.simRecorded = true
	}
}

// Summary holds the aggregated metrics ready for display.
type Summary struct {
	RPCCalls    int
	RPCErrors   int
	RPCTotal    time.Duration
	RPCMin      time.Duration
	RPCMax      time.Duration
	SimDuration time.Duration
	SimRecorded bool
}

// Summarize computes aggregate statistics from the collected records.
func (c *Collector) Summarize() Summary {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := Summary{
		SimDuration: c.simDuration,
		SimRecorded: c.simRecorded,
	}

	for _, r := range c.rpcRecords {
		s.RPCCalls++
		s.RPCTotal += r.Duration
		if r.Err {
			s.RPCErrors++
		}
		if s.RPCMin == 0 || r.Duration < s.RPCMin {
			s.RPCMin = r.Duration
		}
		if r.Duration > s.RPCMax {
			s.RPCMax = r.Duration
		}
	}
	return s
}

// Print writes the performance summary to w (defaults to os.Stdout).
func (c *Collector) Print(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	s := c.Summarize()

	fmt.Fprintln(w, "\n── Performance Summary ──────────────────────────────")
	fmt.Fprintf(w, "  RPC calls     : %d", s.RPCCalls)
	if s.RPCErrors > 0 {
		fmt.Fprintf(w, " (%d errors)", s.RPCErrors)
	}
	fmt.Fprintln(w)
	if s.RPCCalls > 0 {
		fmt.Fprintf(w, "  RPC total     : %s\n", s.RPCTotal.Round(time.Millisecond))
		fmt.Fprintf(w, "  RPC min/max   : %s / %s\n",
			s.RPCMin.Round(time.Millisecond),
			s.RPCMax.Round(time.Millisecond))
		avg := s.RPCTotal / time.Duration(s.RPCCalls)
		fmt.Fprintf(w, "  RPC avg       : %s\n", avg.Round(time.Millisecond))
	}
	if s.SimRecorded {
		fmt.Fprintf(w, "  Replay time   : %s\n", s.SimDuration.Round(time.Millisecond))
	}
	fmt.Fprintln(w, "─────────────────────────────────────────────────────")
}
