package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func withEnv(key, value string, fn func()) {
	orig, had := os.LookupEnv(key)
	if value == "" {
		_ = os.Unsetenv(key)
	} else {
		_ = os.Setenv(key, value)
	}
	defer func() {
		if had {
			_ = os.Setenv(key, orig)
		} else {
			_ = os.Unsetenv(key)
		}
	}()
	fn()
}

func TestDefaultDataDir_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific test")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.UserHomeDir() failed: %v", err)
	}

	// XDG_DATA_HOME should be ignored on darwin
	withEnv("XDG_DATA_HOME", "/tmp/xdg-should-be-ignored", func() {
		got := DefaultDataDir()
		want := filepath.Join(home, "Library", "Application Support", "protolog")

		if got != want {
			t.Fatalf("DefaultDataDir() = %q, want %q", got, want)
		}
	})
}

func TestDefaultDataDir_Unix_XDG(t *testing.T) {
	// This test is for Linux/Unix other than macOS & Windows.
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("Unix (non-darwin) specific test")
	}

	withEnv("XDG_DATA_HOME", "/tmp/xdgtest", func() {
		got := DefaultDataDir()
		want := filepath.Join("/tmp/xdgtest", "protolog")

		if got != want {
			t.Fatalf("DefaultDataDir() with XDG_DATA_HOME = %q, want %q", got, want)
		}
	})
}

func TestDefaultDataDir_Unix_NoXDG(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("Unix (non-darwin) specific test")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.UserHomeDir() failed: %v", err)
	}

	withEnv("XDG_DATA_HOME", "", func() {
		got := DefaultDataDir()
		want := filepath.Join(home, ".local", "share", "protolog")

		if got != want {
			t.Fatalf("DefaultDataDir() = %q, want %q", got, want)
		}
	})
}

func TestDefaultDataDir_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("os.UserHomeDir() failed: %v", err)
	}

	// Case 1: APPDATA set
	withEnv("APPDATA", `C:\AppDataTest`, func() {
		got := DefaultDataDir()
		want := filepath.Join(`C:\AppDataTest`, "protolog")

		if got != want {
			t.Fatalf("DefaultDataDir() with APPDATA = %q, want %q", got, want)
		}
	})

	// Case 2: APPDATA not set: fallback to home\AppData\Roaming\protolog
	withEnv("APPDATA", "", func() {
		got := DefaultDataDir()
		want := filepath.Join(home, "AppData", "Roaming", "protolog")

		if got != want {
			t.Fatalf("DefaultDataDir() with no APPDATA = %q, want %q", got, want)
		}
	})
}
