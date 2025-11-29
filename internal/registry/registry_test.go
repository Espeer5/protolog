package registry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Espeer5/protolog/internal/registry"
	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

// helper to locate schema.desc relative to repo root; assumes tests run from repo root or below.
func descriptorPath() string {
	// Most common case: schema.desc at repo root
	if _, err := os.Stat("schema.desc"); err == nil {
		return "schema.desc"
	}
	// Try up one level (if tests run from ./internal/registry)
	if _, err := os.Stat(filepath.Join("..", "..", "schema.desc")); err == nil {
		return filepath.Join("..", "..", "schema.desc")
	}
	return "schema.desc" // fallback; Load will fail and test will skip
}

func TestNewFromFileAndFormatJSON_LogEnvelope(t *testing.T) {
	descPath := descriptorPath()
	if _, err := os.Stat(descPath); os.IsNotExist(err) {
		t.Skipf("schema descriptor %q not found; run `make proto` first", descPath)
	}

	reg, err := registry.NewFromFile(descPath)
	if err != nil {
		t.Fatalf("NewFromFile(%q) failed: %v", descPath, err)
	}

	// Build a sample LogEnvelope
	env := &logging.LogEnvelope{
		Topic:     "demo",
		Timestamp: timestamppb.Now(),
		Level:     logging.LogLevel_LOG_LEVEL_INFO,
		Host:      "test-host",
		Service:   "test-service",
		Pid:       1234,
		Type:      "logging.LogEnvelope", // we are encoding the envelope itself
		Summary:   "hello world",
	}

	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("proto.Marshal(LogEnvelope) failed: %v", err)
	}

	// Decode using registry as if it were an arbitrary payload
	jsonBytes, err := reg.FormatJSON("logging.LogEnvelope", payload)
	if err != nil {
		t.Fatalf("FormatJSON(logging.LogEnvelope) failed: %v", err)
	}

	// Unmarshal JSON and check a few fields
	var obj map[string]any
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}

	if got := obj["topic"]; got != "demo" {
		t.Errorf("unexpected topic in JSON: got %v, want %q", got, "demo")
	}
	if got := obj["service"]; got != "test-service" {
		t.Errorf("unexpected service in JSON: got %v, want %q", got, "test-service")
	}
	if got := obj["summary"]; got != "hello world" {
		t.Errorf("unexpected summary in JSON: got %v, want %q", got, "hello world")
	}
}

func TestFormatJSON_UnknownType(t *testing.T) {
	descPath := descriptorPath()
	if _, err := os.Stat(descPath); os.IsNotExist(err) {
		t.Skipf("schema descriptor %q not found; run `make proto` first", descPath)
	}

	reg, err := registry.NewFromFile(descPath)
	if err != nil {
		t.Fatalf("NewFromFile(%q) failed: %v", descPath, err)
	}

	_, err = reg.FormatJSON("this.Type.DoesNotExist", []byte{0x00, 0x01})
	if err == nil {
		t.Fatalf("expected error for unknown type, got nil")
	}
}
