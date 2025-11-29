/*******************************************************************************
*  internal/config/paths.go
*
*  This file defines all configured paths for artifacts produced by the logging
*  system.
*******************************************************************************/

package config

/*******************************************************************************
*  IMPORTS
*******************************************************************************/

import (
	"os"
	"path/filepath"
	"runtime"
)

/*******************************************************************************
*  UTILITIES
*******************************************************************************/

// DefaultDataDir returns a per-user application data directory.
// - Linux/Unix: $XDG_DATA_HOME/protolog or ~/.local/share/protolog
// - macOS:      ~/Library/Application Support/protolog
// - Windows:    %AppData%\protolog (if you ever run there)
// - Fallback:   ./data
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// very unusual; fallback to relative dir
		return "data"
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS
		return filepath.Join(home, "Library", "Application Support", "protolog")

	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "protolog")
		}
		return filepath.Join(home, "AppData", "Roaming", "protolog")

	default:
		// Linux / other Unix
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "protolog")
		}
		return filepath.Join(home, ".local", "share", "protolog")
	}
}
