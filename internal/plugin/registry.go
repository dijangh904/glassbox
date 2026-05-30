// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
)

// Registry manages the plugin ecosystem with isolation and versioning
type Registry struct {
	mu       sync.RWMutex
	loader   *Loader
	cache    map[string]json.RawMessage
	bus      *LifecycleBus
	// manifests holds the loaded manifests keyed by plugin name.
	manifests map[string]*Manifest
}

// NewRegistry initializes a fresh registry
func NewRegistry() *Registry {
	return &Registry{
		loader:    NewLoader(),
		cache:     make(map[string]json.RawMessage),
		bus:       NewLifecycleBus(),
		manifests: make(map[string]*Manifest),
	}
}

// Bus returns the lifecycle event bus so callers can subscribe to plugin events.
func (r *Registry) Bus() *LifecycleBus {
	return r.bus
}

// LoadFromDirectory scans and loads all plugins from a directory.
// It first attempts manifest-based discovery (subdirectories with plugin.json).
// If no manifests are found it falls back to scanning for *.so shared libraries.
func (r *Registry) LoadFromDirectory(dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Attempt manifest-based discovery first.
	manifests, manifestErrs := DiscoverManifests(dir)
	if len(manifests) > 0 {
		var loadErrors []error
		for _, m := range manifests {
			manifestDir := filepath.Join(dir, m.Name)
			sp, err := NewSandboxedPlugin(m, manifestDir)
			if err != nil {
				loadErrors = append(loadErrors, fmt.Errorf("plugin %s: %w", m.Name, err))
				continue
			}
			// Run Init lifecycle hook (best-effort).
			if initErr := sp.Init(); initErr != nil {
				r.bus.Emit(LifecyclePayload{
					PluginName: m.Name,
					Event:      EventError,
					Err:        initErr,
				})
			}
			r.loader.plugins[m.Name] = sp
			r.manifests[m.Name] = m
			r.bus.Emit(LifecyclePayload{
				PluginName: m.Name,
				Event:      EventRegistered,
			})
			r.bus.Emit(LifecyclePayload{
				PluginName: m.Name,
				Event:      EventInitialized,
			})
		}
		if len(loadErrors) > 0 {
			return fmt.Errorf("encountered %d plugin loading errors", len(loadErrors))
		}
		return nil
	}

	// Log manifest discovery errors as informational (directory may simply be empty).
	_ = manifestErrs

	// Fallback: scan for *.so shared libraries (original behaviour).
	pattern := filepath.Join(dir, "*.so")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to scan plugin directory: %w", err)
	}

	var loadErrors []error
	for _, path := range matches {
		if err := r.loader.Load(path); err != nil {
			loadErrors = append(loadErrors, err)
		}
	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("encountered %d plugin loading errors", len(loadErrors))
	}

	return nil
}

// RegisterManifest registers a plugin from an explicit manifest path.
// The plugin binary is resolved relative to the manifest's directory.
func (r *Registry) RegisterManifest(manifestPath string) error {
	m, err := LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	manifestDir := filepath.Dir(manifestPath)
	sp, err := NewSandboxedPlugin(m, manifestDir)
	if err != nil {
		return fmt.Errorf("failed to create sandboxed plugin %s: %w", m.Name, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.loader.plugins[m.Name]; exists {
		return fmt.Errorf("plugin %q is already registered", m.Name)
	}

	if initErr := sp.Init(); initErr != nil {
		r.bus.Emit(LifecyclePayload{
			PluginName: m.Name,
			Event:      EventError,
			Err:        initErr,
		})
	}

	r.loader.plugins[m.Name] = sp
	r.manifests[m.Name] = m
	r.bus.Emit(LifecyclePayload{PluginName: m.Name, Event: EventRegistered})
	r.bus.Emit(LifecyclePayload{PluginName: m.Name, Event: EventInitialized})
	return nil
}

// GetManifest returns the manifest for a registered plugin, if available.
func (r *Registry) GetManifest(name string) (*Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.manifests[name]
	return m, ok
}

// Decode uses a plugin to decode an event
func (r *Registry) Decode(pluginName string, eventType string, data []byte) (json.RawMessage, error) {
	r.mu.RLock()

	p, ok := r.loader.Get(pluginName)
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("plugin %s not found", pluginName)
	}

	if !p.CanDecode(eventType) {
		r.mu.RUnlock()
		return nil, fmt.Errorf("plugin %s cannot decode event type %s", pluginName, eventType)
	}

	r.mu.RUnlock()

	result, err := p.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("plugin %s decode failed: %w", pluginName, err)
	}

	return result, nil
}

// FindAndDecode searches for a capable plugin and decodes the event
func (r *Registry) FindAndDecode(eventType string, data []byte) (json.RawMessage, string, error) {
	r.mu.RLock()
	p, ok := r.loader.FindForEvent(eventType)
	r.mu.RUnlock()

	if !ok {
		return nil, "", fmt.Errorf("no plugin available for event type %s", eventType)
	}

	result, err := p.Decode(data)
	if err != nil {
		return nil, "", err
	}

	return result, p.Name(), nil
}

// ListPlugins returns information about all loaded plugins
func (r *Registry) ListPlugins() []Metadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := r.loader.List()
	metadata := make([]Metadata, 0, len(names))

	for _, name := range names {
		if p, ok := r.loader.Get(name); ok {
			metadata = append(metadata, p.Metadata())
		}
	}

	return metadata
}

// Clear removes all loaded plugins and emits cleanup lifecycle events.
func (r *Registry) Clear() {
	r.mu.Lock()
	names := r.loader.List()
	r.loader = NewLoader()
	r.cache = make(map[string]json.RawMessage)
	r.manifests = make(map[string]*Manifest)
	r.mu.Unlock()

	// Emit cleanup events outside the lock.
	for _, name := range names {
		r.bus.Emit(LifecyclePayload{PluginName: name, Event: EventCleanup})
	}
}
