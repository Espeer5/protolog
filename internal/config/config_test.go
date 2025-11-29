package config

import "testing"

func TestDefaultBufferSizePositive(t *testing.T) {
	if DefaultBufferSize <= 0 {
		t.Fatalf("DefaultBufferSize = %d, want > 0", DefaultBufferSize)
	}
}

func TestConfig_BasicFields(t *testing.T) {
	cfg := Config{
		DataDir:    "/tmp/protolog-test",
		BufferSize: 1234,
	}

	if cfg.DataDir != "/tmp/protolog-test" {
		t.Errorf("Config.DataDir = %q, want %q", cfg.DataDir, "/tmp/protolog-test")
	}
	if cfg.BufferSize != 1234 {
		t.Errorf("Config.BufferSize = %d, want %d", cfg.BufferSize, 1234)
	}
}
