// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"sync"
)

// LifecycleEvent identifies a point in a plugin's lifecycle.
type LifecycleEvent string

const (
	// EventRegistered fires after a plugin has been successfully registered.
	EventRegistered LifecycleEvent = "registered"
	// EventInitialized fires after a plugin's Init hook has completed.
	EventInitialized LifecycleEvent = "initialized"
	// EventAnalysisStart fires before a plugin's analysis hook is invoked.
	EventAnalysisStart LifecycleEvent = "analysis_start"
	// EventAnalysisEnd fires after a plugin's analysis hook returns.
	EventAnalysisEnd LifecycleEvent = "analysis_end"
	// EventCleanup fires when a plugin is being unloaded or the registry is cleared.
	EventCleanup LifecycleEvent = "cleanup"
	// EventError fires when a plugin encounters a non-fatal error.
	EventError LifecycleEvent = "error"
)

// LifecyclePayload carries context for a lifecycle event.
type LifecyclePayload struct {
	// PluginName is the name of the plugin that triggered the event.
	PluginName string
	// Event is the lifecycle event type.
	Event LifecycleEvent
	// Err is non-nil for EventError payloads.
	Err error
	// Data holds optional event-specific data.
	Data map[string]any
}

// LifecycleHandler is a function that receives lifecycle events.
type LifecycleHandler func(payload LifecyclePayload)

// LifecycleBus is a concurrency-safe publish/subscribe bus for plugin lifecycle events.
type LifecycleBus struct {
	mu       sync.RWMutex
	handlers map[LifecycleEvent][]LifecycleHandler
}

// NewLifecycleBus returns a ready-to-use LifecycleBus.
func NewLifecycleBus() *LifecycleBus {
	return &LifecycleBus{
		handlers: make(map[LifecycleEvent][]LifecycleHandler),
	}
}

// On registers a handler for the given lifecycle event.
func (b *LifecycleBus) On(event LifecycleEvent, handler LifecycleHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[event] = append(b.handlers[event], handler)
}

// Emit delivers a payload to all handlers registered for the event.
// Handlers are called synchronously; panics inside handlers are recovered
// and silently discarded to prevent a misbehaving observer from crashing
// the host process.
func (b *LifecycleBus) Emit(payload LifecyclePayload) {
	b.mu.RLock()
	handlers := make([]LifecycleHandler, len(b.handlers[payload.Event]))
	copy(handlers, b.handlers[payload.Event])
	b.mu.RUnlock()

	for _, h := range handlers {
		safeCall(h, payload)
	}
}

// safeCall invokes h(payload) and recovers from any panic so that a
// misbehaving lifecycle observer cannot crash the host process.
func safeCall(h LifecycleHandler, payload LifecyclePayload) {
	defer func() { recover() }() //nolint:errcheck
	h(payload)
}
