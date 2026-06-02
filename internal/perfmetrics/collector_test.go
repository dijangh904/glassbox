// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package perfmetrics

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCollector_RecordRPC(t *testing.T) {
	c := NewCollector()
	c.RecordRPC("getTransaction", 120*time.Millisecond, false)
	c.RecordRPC("getLedgerEntries", 80*time.Millisecond, false)
	c.RecordRPC("simulateTransaction", 200*time.Millisecond, true)

	s := c.Summarize()
	assert.Equal(t, 3, s.RPCCalls)
	assert.Equal(t, 1, s.RPCErrors)
	assert.Equal(t, 400*time.Millisecond, s.RPCTotal)
	assert.Equal(t, 80*time.Millisecond, s.RPCMin)
	assert.Equal(t, 200*time.Millisecond, s.RPCMax)
}

func TestCollector_SimTiming(t *testing.T) {
	c := NewCollector()
	c.StartSim()
	time.Sleep(5 * time.Millisecond)
	c.StopSim()

	s := c.Summarize()
	assert.True(t, s.SimRecorded)
	assert.True(t, s.SimDuration >= 5*time.Millisecond)
}

func TestCollector_StopSim_WithoutStart(t *testing.T) {
	c := NewCollector()
	c.StopSim() // should not panic
	s := c.Summarize()
	assert.False(t, s.SimRecorded)
}

func TestCollector_Print(t *testing.T) {
	c := NewCollector()
	c.RecordRPC("getTransaction", 100*time.Millisecond, false)
	c.RecordRPC("getLedgerEntries", 50*time.Millisecond, true)
	c.StartSim()
	c.StopSim()

	var buf bytes.Buffer
	c.Print(&buf)
	out := buf.String()

	assert.True(t, strings.Contains(out, "RPC calls"), "should contain RPC calls line")
	assert.True(t, strings.Contains(out, "errors"), "should mention errors")
	assert.True(t, strings.Contains(out, "Replay time"), "should contain replay time")
}

func TestCollector_Print_NoRPC(t *testing.T) {
	c := NewCollector()
	var buf bytes.Buffer
	c.Print(&buf)
	out := buf.String()
	assert.True(t, strings.Contains(out, "RPC calls"), "header should still appear")
	assert.False(t, strings.Contains(out, "RPC total"), "no total when no calls")
}
