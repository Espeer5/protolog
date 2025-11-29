package storage

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

// helper to read a single length-prefixed LogEnvelope from a file
func readSingleEnvelope(t *testing.T, path string) *logging.LogEnvelope {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", path, err)
	}
	if len(data) < 4 {
		t.Fatalf("file %q too short: len=%d", path, len(data))
	}

	n := binary.BigEndian.Uint32(data[:4])
	if int(n) != len(data[4:]) {
		t.Fatalf("length prefix %d does not match payload len %d", n, len(data[4:]))
	}

	var env logging.LogEnvelope
	if err := proto.Unmarshal(data[4:], &env); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}
	return &env
}

func TestNewWriter_CreatesBaseDir(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "logs")

	w, err := NewWriter(base)
	if err != nil {
		t.Fatalf("NewWriter(%q) returned error: %v", base, err)
	}
	t.Cleanup(func() { _ = w.Close() })

	if st, err := os.Stat(base); err != nil {
		t.Fatalf("expected base dir %q to exist, got error: %v", base, err)
	} else if !st.IsDir() {
		t.Fatalf("expected %q to be a directory", base)
	}
}

func TestWriter_WritesPerTopicFiles(t *testing.T) {
	base := t.TempDir()

	w, err := NewWriter(base)
	if err != nil {
		t.Fatalf("NewWriter(%q) returned error: %v", base, err)
	}
	t.Cleanup(func() { _ = w.Close() })

	env1 := &logging.LogEnvelope{
		Topic:     "alpha",
		Summary:   "hello alpha",
		Timestamp: timestamppb.Now(),
		Level:     logging.LogLevel_LOG_LEVEL_INFO,
		Host:      "host-a",
		Service:   "svc-a",
	}
	env2 := &logging.LogEnvelope{
		Topic:     "beta",
		Summary:   "hello beta",
		Timestamp: timestamppb.Now(),
		Level:     logging.LogLevel_LOG_LEVEL_WARN,
		Host:      "host-b",
		Service:   "svc-b",
	}

	if err := w.WriteEnvelope(env1); err != nil {
		t.Fatalf("WriteEnvelope(alpha) returned error: %v", err)
	}
	if err := w.WriteEnvelope(env2); err != nil {
		t.Fatalf("WriteEnvelope(beta) returned error: %v", err)
	}

	alphaPath := filepath.Join(base, "alpha.log")
	betaPath := filepath.Join(base, "beta.log")

	if _, err := os.Stat(alphaPath); err != nil {
		t.Fatalf("expected alpha log file %q to exist: %v", alphaPath, err)
	}
	if _, err := os.Stat(betaPath); err != nil {
		t.Fatalf("expected beta log file %q to exist: %v", betaPath, err)
	}

	gotAlpha := readSingleEnvelope(t, alphaPath)
	if gotAlpha.GetTopic() != "alpha" || gotAlpha.GetSummary() != "hello alpha" {
		t.Fatalf("alpha envelope mismatch: got topic=%q summary=%q, want topic=%q summary=%q",
			gotAlpha.GetTopic(), gotAlpha.GetSummary(), "alpha", "hello alpha")
	}

	gotBeta := readSingleEnvelope(t, betaPath)
	if gotBeta.GetTopic() != "beta" || gotBeta.GetSummary() != "hello beta" {
		t.Fatalf("beta envelope mismatch: got topic=%q summary=%q, want topic=%q summary=%q",
			gotBeta.GetTopic(), gotBeta.GetSummary(), "beta", "hello beta")
	}
}

func TestWriter_EmptyTopicUsesUntitledFile(t *testing.T) {
	base := t.TempDir()

	w, err := NewWriter(base)
	if err != nil {
		t.Fatalf("NewWriter(%q) returned error: %v", base, err)
	}
	t.Cleanup(func() { _ = w.Close() })

	env := &logging.LogEnvelope{
		Topic:   "",
		Summary: "no topic",
	}

	if err := w.WriteEnvelope(env); err != nil {
		t.Fatalf("WriteEnvelope(empty topic) returned error: %v", err)
	}

	path := filepath.Join(base, "untitled.log")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected untitled.log to exist at %q: %v", path, err)
	}

	got := readSingleEnvelope(t, path)
	if got.GetTopic() != "" {
		t.Fatalf("expected stored envelope topic to be empty, got %q", got.GetTopic())
	}
	if got.GetSummary() != "no topic" {
		t.Fatalf("summary mismatch: got %q, want %q", got.GetSummary(), "no topic")
	}
}

func TestWriter_AppendsMultipleEnvelopes(t *testing.T) {
	base := t.TempDir()

	w, err := NewWriter(base)
	if err != nil {
		t.Fatalf("NewWriter(%q) returned error: %v", base, err)
	}
	t.Cleanup(func() { _ = w.Close() })

	env1 := &logging.LogEnvelope{Topic: "demo", Summary: "first"}
	env2 := &logging.LogEnvelope{Topic: "demo", Summary: "second"}

	if err := w.WriteEnvelope(env1); err != nil {
		t.Fatalf("WriteEnvelope(first) error: %v", err)
	}
	if err := w.WriteEnvelope(env2); err != nil {
		t.Fatalf("WriteEnvelope(second) error: %v", err)
	}

	path := filepath.Join(base, "demo.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", path, err)
	}

	// Parse two records
	offset := 0
	readEnv := func() *logging.LogEnvelope {
		if len(data[offset:]) < 4 {
			t.Fatalf("not enough bytes for length prefix")
		}
		n := binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
		if len(data[offset:]) < int(n) {
			t.Fatalf("length prefix %d exceeds remaining %d", n, len(data[offset:]))
		}
		var e logging.LogEnvelope
		if err := proto.Unmarshal(data[offset:offset+int(n)], &e); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		offset += int(n)
		return &e
	}

	got1 := readEnv()
	got2 := readEnv()

	if got1.GetSummary() != "first" || got2.GetSummary() != "second" {
		t.Fatalf("unexpected summaries in file: got [%q, %q], want [first, second]",
			got1.GetSummary(), got2.GetSummary())
	}
	if offset != len(data) {
		t.Fatalf("expected to consume all bytes, offset=%d len=%d", offset, len(data))
	}
}
