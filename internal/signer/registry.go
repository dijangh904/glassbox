// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package signer

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry is a thread-safe map of provider name → SignerProvider.
// Use DefaultRegistry for the global instance, or create a new Registry
// for isolated testing.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]SignerProvider
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]SignerProvider)}
}

// DefaultRegistry is the process-wide provider registry. The built-in
// "software" and "pkcs11" providers are registered at package init time.
var DefaultRegistry = NewRegistry()

func init() {
	DefaultRegistry.Register(&SoftwareProvider{})
	DefaultRegistry.Register(&PKCS11Provider{})
}

// Register adds a provider to the registry. It panics if a provider with
// the same name is already registered, matching the behaviour of
// database/sql.Register and http.Handle.
// Provider names are normalised to lowercase before storage.
func (r *Registry) Register(p SignerProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := strings.ToLower(p.Name())
	if _, exists := r.providers[name]; exists {
		panic(fmt.Sprintf("signer: provider %q already registered", name))
	}
	r.providers[name] = p
}

// RegisterOrReplace adds or replaces a provider. Intended for testing.
// Provider names are normalised to lowercase before storage.
func (r *Registry) RegisterOrReplace(p SignerProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[strings.ToLower(p.Name())] = p
}

// Get returns the provider registered under name, or an error if not found.
// The lookup is case-insensitive.
func (r *Registry) Get(name string) (SignerProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[strings.ToLower(name)]
	if !ok {
		return nil, &Error{
			Op:  "registry",
			Msg: fmt.Sprintf("unknown signing provider %q; available: %s", name, strings.Join(r.names(), ", ")),
		}
	}
	return p, nil
}

// Names returns the sorted list of registered provider names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.names()
}

// names is the unlocked variant for internal use.
func (r *Registry) names() []string {
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// CreateSigner resolves the provider by name, validates cfg, and returns a
// ready-to-use Signer. It is the primary entry point for the CLI layer.
func (r *Registry) CreateSigner(providerName string, cfg ProviderConfig) (Signer, error) {
	p, err := r.Get(providerName)
	if err != nil {
		return nil, err
	}
	if err := p.Validate(cfg); err != nil {
		return nil, &Error{Op: "registry", Msg: fmt.Sprintf("provider %q: invalid configuration", providerName), Err: err}
	}
	return p.Create(cfg)
}
