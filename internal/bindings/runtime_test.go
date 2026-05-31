// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package bindings

import (
	"strings"
	"testing"
)

// buildTestGeneratorWithTarget returns a Generator pre-loaded with a mock spec
// and the given RuntimeTarget.
func buildTestGeneratorWithTarget(target RuntimeTarget) *Generator {
	g := buildTestGenerator()
	g.config.RuntimeTarget = target
	return g
}

// ─── RuntimeTarget defaults ───────────────────────────────────────────────────

func TestRuntimeTarget_Default(t *testing.T) {
	g := buildTestGenerator()
	g.config.RuntimeTarget = ""
	if g.runtimeTarget() != RuntimeNode {
		t.Errorf("expected default RuntimeNode, got %q", g.runtimeTarget())
	}
}

// ─── Node target ──────────────────────────────────────────────────────────────

func TestGenerateClient_NodeTarget_UsesSpawn(t *testing.T) {
	g := buildTestGeneratorWithTarget(RuntimeNode)
	client := g.generateClient()

	// Node target should NOT emit _pickFetch.
	if strings.Contains(client, "_pickFetch") {
		t.Error("node target should not emit _pickFetch")
	}
	// Node target uses serverOpts pattern.
	if !strings.Contains(client, "serverOpts") {
		t.Error("node target should use serverOpts for optional fetch")
	}
}

func TestGenerateErstIntegration_NodeTarget(t *testing.T) {
	g := buildTestGeneratorWithTarget(RuntimeNode)
	integration := g.generateErstIntegration()

	if !strings.Contains(integration, "import { spawn } from 'child_process';") {
		t.Error("node target should import child_process")
	}
	if strings.Contains(integration, "runHTTP") {
		t.Error("node target should not emit runHTTP")
	}
	if !strings.Contains(integration, "runGlassbox") {
		t.Error("node target should emit runGlassbox")
	}
}

// ─── Browser target ───────────────────────────────────────────────────────────

func TestGenerateClient_BrowserTarget_NoNodeGlobals(t *testing.T) {
	g := buildTestGeneratorWithTarget(RuntimeBrowser)
	client := g.generateClient()

	// Browser target must not reference child_process or Node-only APIs.
	if strings.Contains(client, "child_process") {
		t.Error("browser target must not reference child_process")
	}
	if strings.Contains(client, "require(") {
		t.Error("browser target must not use require()")
	}
	// Browser target uses globalThis.fetch.
	if !strings.Contains(client, "globalThis.fetch") {
		t.Error("browser target should use globalThis.fetch")
	}
}

func TestGenerateErstIntegration_BrowserTarget_NoChildProcess(t *testing.T) {
	g := buildTestGeneratorWithTarget(RuntimeBrowser)
	integration := g.generateErstIntegration()

	if strings.Contains(integration, "child_process") {
		t.Error("browser target must not import child_process")
	}
	if strings.Contains(integration, "spawn(") {
		t.Error("browser target must not use spawn()")
	}
	if !strings.Contains(integration, "runHTTP") {
		t.Error("browser target should emit runHTTP")
	}
	if !strings.Contains(integration, "globalThis.fetch") {
		t.Error("browser target should use globalThis.fetch in runHTTP")
	}
}

func TestGeneratePackageJSON_BrowserTarget_HasBrowserField(t *testing.T) {
	g := buildTestGeneratorWithTarget(RuntimeBrowser)
	pkg := g.generatePackageJSON()

	if !strings.Contains(pkg, `"browser"`) {
		t.Error("browser target package.json should have browser field")
	}
	if !strings.Contains(pkg, `"child_process": false`) {
		t.Error("browser target should exclude child_process")
	}
}

func TestGenerateReadme_BrowserTarget_HasBrowserSection(t *testing.T) {
	g := buildTestGeneratorWithTarget(RuntimeBrowser)
	readme := g.generateReadme()

	if !strings.Contains(readme, "Browser Usage") {
		t.Error("browser target README should have Browser Usage section")
	}
	if !strings.Contains(readme, "Runtime target:** Browser") {
		t.Error("browser target README should mention browser runtime")
	}
}

// ─── Universal target ─────────────────────────────────────────────────────────

func TestGenerateClient_UniversalTarget_HasEnvDetection(t *testing.T) {
	g := buildTestGeneratorWithTarget(RuntimeUniversal)
	client := g.generateClient()

	if !strings.Contains(client, "_pickFetch") {
		t.Error("universal target should emit _pickFetch")
	}
	if !strings.Contains(client, "globalThis") {
		t.Error("universal target should reference globalThis")
	}
}

func TestGenerateErstIntegration_UniversalTarget_HasBothPaths(t *testing.T) {
	g := buildTestGeneratorWithTarget(RuntimeUniversal)
	integration := g.generateErstIntegration()

	if !strings.Contains(integration, "runHTTP") {
		t.Error("universal target should emit runHTTP for browser path")
	}
	if !strings.Contains(integration, "runGlassbox") {
		t.Error("universal target should emit runGlassbox for node path")
	}
	if !strings.Contains(integration, "_isNode()") {
		t.Error("universal target should emit _isNode() guard")
	}
	// child_process should be loaded lazily via require, not imported at top.
	if strings.Contains(integration, "import { spawn } from 'child_process';") {
		t.Error("universal target should not statically import child_process")
	}
	if !strings.Contains(integration, "require('child_process')") {
		t.Error("universal target should lazily require child_process")
	}
}

func TestGeneratePackageJSON_UniversalTarget_HasExports(t *testing.T) {
	g := buildTestGeneratorWithTarget(RuntimeUniversal)
	pkg := g.generatePackageJSON()

	if !strings.Contains(pkg, `"exports"`) {
		t.Error("universal target package.json should have exports field")
	}
	if !strings.Contains(pkg, `"browser"`) {
		t.Error("universal target exports should have browser condition")
	}
}

// ─── Custom provider ──────────────────────────────────────────────────────────

func TestGenerateClient_CustomProvider_Interface(t *testing.T) {
	for _, target := range []RuntimeTarget{RuntimeNode, RuntimeBrowser, RuntimeUniversal} {
		t.Run(string(target), func(t *testing.T) {
			g := buildTestGeneratorWithTarget(target)
			client := g.generateClient()

			if !strings.Contains(client, "export interface SorobanProvider") {
				t.Error("SorobanProvider interface should be emitted")
			}
			if !strings.Contains(client, "provider?: SorobanProvider;") {
				t.Error("ClientConfig should have optional provider field")
			}
			if !strings.Contains(client, "config.provider?.rpcUrl") {
				t.Error("constructor should prefer provider.rpcUrl")
			}
			if !strings.Contains(client, "config.provider?.networkPassphrase") {
				t.Error("getNetworkPassphrase should check provider.networkPassphrase")
			}
		})
	}
}
