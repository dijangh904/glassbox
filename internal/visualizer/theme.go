// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"os"
	"strconv"
	"strings"
)

// Theme defines a color palette for terminal output
type Theme string

const (
	ThemeDefault      Theme = "default"
	ThemeDeuteranopia Theme = "deuteranopia"
	ThemeProtanopia   Theme = "protanopia"
	ThemeTritanopia   Theme = "tritanopia"
	ThemeHighContrast Theme = "high-contrast"
	ThemeLight        Theme = "light"
	ThemeDark         Theme = "dark"
)

var currentTheme = ThemeDefault

// SetTheme configures the active color theme
func SetTheme(theme Theme) {
	currentTheme = theme
}

// GetTheme returns the currently active theme
func GetTheme() Theme {
	return currentTheme
}

// DetectTheme attempts to detect an appropriate theme from environment.
// Detection order:
//  1. GLASSBOX_THEME env var (explicit user override)
//  2. COLORFGBG env var (set by many terminals; encodes fg/bg color indices)
//  3. COLORTERM=truecolor (true-colour capable terminal → default palette)
//  4. High-contrast fallback for limited-colour environments
func DetectTheme() Theme {
	if theme := os.Getenv("GLASSBOX_THEME"); theme != "" {
		switch theme {
		case "light":
			return ThemeLight
		case "dark":
			return ThemeDark
		default:
			return Theme(theme)
		}
	}
	// COLORFGBG is exported by rxvt, konsole, and other terminals as "fg;bg"
	// or "fg;unused;bg".  A background index >= 8 means a bright (light) color.
	if colorfgbg := os.Getenv("COLORFGBG"); colorfgbg != "" {
		if terminalBackgroundIsLight(colorfgbg) {
			return ThemeLight
		}
		return ThemeDark
	}
	if os.Getenv("COLORTERM") == "truecolor" {
		return ThemeDefault
	}
	return ThemeHighContrast
}

// terminalBackgroundIsLight parses a COLORFGBG value (format "fg;bg" or "fg;mid;bg")
// and returns true when the background color index indicates a light background.
func terminalBackgroundIsLight(colorfgbg string) bool {
	parts := strings.Split(colorfgbg, ";")
	if len(parts) == 0 {
		return false
	}
	bg := strings.TrimSpace(parts[len(parts)-1])
	n, err := strconv.Atoi(bg)
	if err != nil {
		return false
	}
	// ANSI indices 8–15 are the bright/light variants (15 = white).
	return n >= 8
}

// themeColors maps semantic color names to ANSI codes per theme
func themeColors(semantic string) string {
	switch currentTheme {
	case ThemeLight:
		switch semantic {
		case "success":
			return sgrBold + sgrGreen
		case "error":
			return sgrBold + sgrRed
		case "warning":
			return sgrBold + "\033[38;5;130m"
		case "info":
			return sgrBold + sgrBlue
		case "dim":
			return "\033[38;5;240m"
		case "bold":
			return sgrBold
		default:
			return ""
		}
	case ThemeDark:
		switch semantic {
		case "success":
			return "\033[38;5;46m"
		case "error":
			return "\033[38;5;196m"
		case "warning":
			return "\033[38;5;226m"
		case "info":
			return "\033[38;5;51m"
		case "dim":
			return "\033[38;5;244m"
		case "bold":
			return sgrBold
		default:
			return ""
		}
	case ThemeDeuteranopia, ThemeProtanopia:
		// Red-green color blindness: use blue/yellow/cyan
		switch semantic {
		case "success":
			return sgrCyan
		case "error":
			return sgrMagenta
		case "warning":
			return sgrYellow
		case "info":
			return sgrBlue
		case "dim":
			return sgrDim
		case "bold":
			return sgrBold
		default:
			return ""
		}
	case ThemeTritanopia:
		// Blue-yellow color blindness: use red/green/magenta
		switch semantic {
		case "success":
			return sgrGreen
		case "error":
			return sgrRed
		case "warning":
			return sgrMagenta
		case "info":
			return sgrCyan
		case "dim":
			return sgrDim
		case "bold":
			return sgrBold
		default:
			return ""
		}
	case ThemeHighContrast:
		// High contrast: bold colors only
		switch semantic {
		case "success":
			return sgrBold + sgrGreen
		case "error":
			return sgrBold + sgrRed
		case "warning":
			return sgrBold + sgrYellow
		case "info":
			return sgrBold + sgrCyan
		case "dim":
			return ""
		case "bold":
			return sgrBold
		default:
			return ""
		}
	default:
		// Default theme
		switch semantic {
		case "success":
			return sgrGreen
		case "error":
			return sgrRed
		case "warning":
			return sgrYellow
		case "info":
			return sgrBlue
		case "dim":
			return sgrDim
		case "bold":
			return sgrBold
		default:
			return ""
		}
	}
}
