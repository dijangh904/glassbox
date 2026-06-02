// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"os"
	"testing"
)

func TestSetTheme(t *testing.T) {
	tests := []struct {
		name  string
		theme Theme
		want  Theme
	}{
		{"default", ThemeDefault, ThemeDefault},
		{"deuteranopia", ThemeDeuteranopia, ThemeDeuteranopia},
		{"protanopia", ThemeProtanopia, ThemeProtanopia},
		{"tritanopia", ThemeTritanopia, ThemeTritanopia},
		{"high-contrast", ThemeHighContrast, ThemeHighContrast},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTheme(tt.theme)
			if got := GetTheme(); got != tt.want {
				t.Errorf("GetTheme() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectTheme(t *testing.T) {
	tests := []struct {
		name       string
		envTheme   string
		colorTerm  string
		colorFGBG  string
		want       Theme
	}{
		{"explicit theme", "deuteranopia", "", "", ThemeDeuteranopia},
		{"explicit dark", "dark", "", "", ThemeDark},
		{"explicit light", "light", "", "", ThemeLight},
		{"truecolor", "", "truecolor", "", ThemeDefault},
		{"colorfgbg dark bg", "", "", "15;0", ThemeDark},
		{"colorfgbg light bg", "", "", "0;15", ThemeLight},
		{"colorfgbg three-part dark", "", "", "15;default;0", ThemeDark},
		{"fallback", "", "", "", ThemeHighContrast},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Unsetenv("GLASSBOX_THEME")
			_ = os.Unsetenv("COLORTERM")
			_ = os.Unsetenv("COLORFGBG")

			if tt.envTheme != "" {
				_ = os.Setenv("GLASSBOX_THEME", tt.envTheme)
			}
			if tt.colorTerm != "" {
				_ = os.Setenv("COLORTERM", tt.colorTerm)
			}
			if tt.colorFGBG != "" {
				_ = os.Setenv("COLORFGBG", tt.colorFGBG)
			}

			if got := DetectTheme(); got != tt.want {
				t.Errorf("DetectTheme() = %v, want %v", got, tt.want)
			}

			_ = os.Unsetenv("GLASSBOX_THEME")
			_ = os.Unsetenv("COLORTERM")
			_ = os.Unsetenv("COLORFGBG")
		})
	}
}

func TestTerminalBackgroundIsLight(t *testing.T) {
	tests := []struct {
		colorfgbg string
		want      bool
	}{
		{"15;0", false},   // black background (index 0) → dark
		{"0;15", true},    // white background (index 15) → light
		{"0;7", false},    // dark gray (index 7) → dark
		{"0;8", true},     // bright black / dark gray (index 8) → light boundary
		{"15;default;0", false}, // three-part with non-numeric → dark (default)
		{"0;12", true},    // bright blue (index 12) → light
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		got := terminalBackgroundIsLight(tt.colorfgbg)
		if got != tt.want {
			t.Errorf("terminalBackgroundIsLight(%q) = %v, want %v", tt.colorfgbg, got, tt.want)
		}
	}
}

func TestThemeColors(t *testing.T) {
	tests := []struct {
		name     string
		theme    Theme
		semantic string
		wantCode string
	}{
		{"default success", ThemeDefault, "success", sgrGreen},
		{"default error", ThemeDefault, "error", sgrRed},
		{"default warning", ThemeDefault, "warning", sgrYellow},
		{"dark success", ThemeDark, "success", "\033[38;5;46m"},
		{"dark error", ThemeDark, "error", "\033[38;5;196m"},
		{"dark warning", ThemeDark, "warning", "\033[38;5;226m"},
		{"dark info", ThemeDark, "info", "\033[38;5;51m"},
		{"light success", ThemeLight, "success", sgrBold + sgrGreen},
		{"light error", ThemeLight, "error", sgrBold + sgrRed},
		{"light warning", ThemeLight, "warning", sgrBold + "\033[38;5;130m"},
		{"light info", ThemeLight, "info", sgrBold + sgrBlue},
		{"deuteranopia success", ThemeDeuteranopia, "success", sgrCyan},
		{"deuteranopia error", ThemeDeuteranopia, "error", sgrMagenta},
		{"protanopia success", ThemeProtanopia, "success", sgrCyan},
		{"tritanopia success", ThemeTritanopia, "success", sgrGreen},
		{"tritanopia warning", ThemeTritanopia, "warning", sgrMagenta},
		{"high-contrast success", ThemeHighContrast, "success", sgrBold + sgrGreen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTheme(tt.theme)
			if got := themeColors(tt.semantic); got != tt.wantCode {
				t.Errorf("themeColors(%q) = %q, want %q", tt.semantic, got, tt.wantCode)
			}
		})
	}
}

func TestThemeAwareIndicators(t *testing.T) {
	originalTheme := GetTheme()
	defer SetTheme(originalTheme)

	_ = os.Setenv("FORCE_COLOR", "1")
	defer func() { _ = os.Unsetenv("FORCE_COLOR") }()

	tests := []struct {
		name  string
		theme Theme
		fn    func() string
	}{
		{"success default", ThemeDefault, Success},
		{"success deuteranopia", ThemeDeuteranopia, Success},
		{"error default", ThemeDefault, Error},
		{"warning high-contrast", ThemeHighContrast, Warning},
		{"info default", ThemeDefault, Info},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTheme(tt.theme)
			result := tt.fn()
			if result == "" {
				t.Error("indicator returned empty string")
			}
			if !ColorEnabled() {
				t.Skip("colors disabled")
			}
		})
	}
}
