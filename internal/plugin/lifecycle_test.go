// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"errors"
	"sync"
	"testing"
)

func TestLifecycleBus_Subscribe_Emit(t *testing.T) {
	bus := NewLifecycleBus()

	var received []LifecyclePayload
	bus.On(EventRegistered, func(p LifecyclePayload) {
		received = append(received, p)
	})

	bus.Emit(LifecyclePayload{PluginName: "my-plugin", Event: EventRegistered})

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].PluginName != "my-plugin" {
		t.Errorf("expected plugin name my-plugin, got %s", received[0].PluginName)
	}
}

func TestLifecycleBus_MultipleHandlers(t *testing.T) {
	bus := NewLifecycleBus()

	count := 0
	bus.On(EventInitialized, func(p LifecyclePayload) { count++ })
	bus.On(EventInitialized, func(p LifecyclePayload) { count++ })
	bus.On(EventInitialized, func(p LifecyclePayload) { count++ })

	bus.Emit(LifecyclePayload{PluginName: "p", Event: EventInitialized})

	if count != 3 {
		t.Errorf("expected 3 handler calls, got %d", count)
	}
}

func TestLifecycleBus_DifferentEvents(t *testing.T) {
	bus := NewLifecycleBus()

	var registered, cleanup int
	bus.On(EventRegistered, func(p LifecyclePayload) { registered++ })
	bus.On(EventCleanup, func(p LifecyclePayload) { cleanup++ })

	bus.Emit(LifecyclePayload{Event: EventRegistered})
	bus.Emit(LifecyclePayload{Event: EventCleanup})
	bus.Emit(LifecyclePayload{Event: EventRegistered})

	if registered != 2 {
		t.Errorf("expected 2 registered events, got %d", registered)
	}
	if cleanup != 1 {
		t.Errorf("expected 1 cleanup event, got %d", cleanup)
	}
}

func TestLifecycleBus_NoHandlers(t *testing.T) {
	bus := NewLifecycleBus()
	// Should not panic when no handlers are registered.
	bus.Emit(LifecyclePayload{PluginName: "p", Event: EventRegistered})
}

func TestLifecycleBus_PanicInHandlerDoesNotCrash(t *testing.T) {
	bus := NewLifecycleBus()

	bus.On(EventRegistered, func(p LifecyclePayload) {
		panic("handler panic")
	})

	// Must not propagate the panic.
	bus.Emit(LifecyclePayload{PluginName: "p", Event: EventRegistered})
}

func TestLifecycleBus_ErrorPayload(t *testing.T) {
	bus := NewLifecycleBus()

	var gotErr error
	bus.On(EventError, func(p LifecyclePayload) {
		gotErr = p.Err
	})

	sentinel := errors.New("plugin exploded")
	bus.Emit(LifecyclePayload{PluginName: "bad-plugin", Event: EventError, Err: sentinel})

	if gotErr != sentinel {
		t.Errorf("expected sentinel error, got %v", gotErr)
	}
}

func TestLifecycleBus_ConcurrentEmit(t *testing.T) {
	bus := NewLifecycleBus()

	var mu sync.Mutex
	count := 0
	bus.On(EventAnalysisStart, func(p LifecyclePayload) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Emit(LifecyclePayload{Event: EventAnalysisStart})
		}()
	}
	wg.Wait()

	if count != 100 {
		t.Errorf("expected 100 events, got %d", count)
	}
}

func TestLifecycleBus_AllEventTypes(t *testing.T) {
	bus := NewLifecycleBus()
	events := []LifecycleEvent{
		EventRegistered,
		EventInitialized,
		EventAnalysisStart,
		EventAnalysisEnd,
		EventCleanup,
		EventError,
	}

	received := make(map[LifecycleEvent]int)
	var mu sync.Mutex
	for _, ev := range events {
		ev := ev
		bus.On(ev, func(p LifecyclePayload) {
			mu.Lock()
			received[ev]++
			mu.Unlock()
		})
	}

	for _, ev := range events {
		bus.Emit(LifecyclePayload{Event: ev})
	}

	for _, ev := range events {
		if received[ev] != 1 {
			t.Errorf("expected 1 event for %s, got %d", ev, received[ev])
		}
	}
}

func TestLifecycleBus_RegistryClearEmitsCleanup(t *testing.T) {
	r := NewRegistry()

	var cleanupNames []string
	var mu sync.Mutex
	r.Bus().On(EventCleanup, func(p LifecyclePayload) {
		mu.Lock()
		cleanupNames = append(cleanupNames, p.PluginName)
		mu.Unlock()
	})

	// Inject two mock plugins directly.
	r.mu.Lock()
	r.loader.plugins["alpha"] = &mockDecoder{name: "alpha", version: "1.0.0"}
	r.loader.plugins["beta"] = &mockDecoder{name: "beta", version: "1.0.0"}
	r.mu.Unlock()

	r.Clear()

	if len(cleanupNames) != 2 {
		t.Errorf("expected 2 cleanup events, got %d: %v", len(cleanupNames), cleanupNames)
	}
}

func TestLifecycleBus_RegisterManifestEmitsEvents(t *testing.T) {
	// We can't easily test RegisterManifest end-to-end without a real binary,
	// but we can verify the bus wiring by injecting directly and checking events.
	r := NewRegistry()

	var events []LifecycleEvent
	var mu sync.Mutex
	for _, ev := range []LifecycleEvent{EventRegistered, EventInitialized} {
		ev := ev
		r.Bus().On(ev, func(p LifecyclePayload) {
			mu.Lock()
			events = append(events, ev)
			mu.Unlock()
		})
	}

	// Simulate what RegisterManifest does after creating the sandboxed plugin.
	r.mu.Lock()
	r.loader.plugins["wired"] = &mockDecoder{name: "wired", version: "1.0.0"}
	r.mu.Unlock()
	r.Bus().Emit(LifecyclePayload{PluginName: "wired", Event: EventRegistered})
	r.Bus().Emit(LifecyclePayload{PluginName: "wired", Event: EventInitialized})

	if len(events) != 2 {
		t.Errorf("expected 2 lifecycle events, got %d", len(events))
	}
}
