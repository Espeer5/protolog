package memory

import (
	"reflect"
	"sort"
	"testing"

	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

// helper to make a simple envelope with recognizable fields
func makeEnv(topic, summary string) *logging.LogEnvelope {
	return &logging.LogEnvelope{
		Topic:   topic,
		Summary: summary,
	}
}

func summaries(envs []*logging.LogEnvelope) []string {
	out := make([]string, 0, len(envs))
	for _, e := range envs {
		out = append(out, e.GetSummary())
	}
	return out
}

// --- RingBuffer tests ---

func TestRingBuffer_AddAndRecentOrder(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Add(makeEnv("demo", "one"))
	rb.Add(makeEnv("demo", "two"))
	rb.Add(makeEnv("demo", "three"))

	got := summaries(rb.Recent(10)) // ask for more than size to test clamping
	want := []string{"three", "two", "one"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Recent(10) = %v, want %v", got, want)
	}
}

func TestRingBuffer_OverwriteOldest(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Add(makeEnv("demo", "one")) // will be overwritten
	rb.Add(makeEnv("demo", "two"))
	rb.Add(makeEnv("demo", "three"))
	rb.Add(makeEnv("demo", "four")) // overwrites "one"
	rb.Add(makeEnv("demo", "five")) // overwrites "two"

	got := summaries(rb.Recent(10))
	want := []string{"five", "four", "three"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Recent(10) = %v, want %v", got, want)
	}
}

func TestRingBuffer_RecentLimit(t *testing.T) {
	rb := NewRingBuffer(5)

	rb.Add(makeEnv("demo", "one"))
	rb.Add(makeEnv("demo", "two"))
	rb.Add(makeEnv("demo", "three"))
	rb.Add(makeEnv("demo", "four"))
	rb.Add(makeEnv("demo", "five"))

	got := summaries(rb.Recent(2))
	want := []string{"five", "four"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Recent(2) = %v, want %v", got, want)
	}
}

// --- TopicBuffers tests ---

func TestTopicBuffers_PerTopicIsolation(t *testing.T) {
	tb := NewTopicBuffers(2) // each topic buffer can hold 2

	tb.Add(makeEnv("alpha", "a1"))
	tb.Add(makeEnv("alpha", "a2"))
	tb.Add(makeEnv("beta", "b1"))
	tb.Add(makeEnv("beta", "b2"))
	tb.Add(makeEnv("alpha", "a3")) // alpha overwrites a1 (size 2)

	gotAlpha := summaries(tb.Recent("alpha", 10))
	gotBeta := summaries(tb.Recent("beta", 10))

	wantAlpha := []string{"a3", "a2"} // newest-first, only last 2 kept
	wantBeta := []string{"b2", "b1"}

	if !reflect.DeepEqual(gotAlpha, wantAlpha) {
		t.Fatalf("Recent(alpha) = %v, want %v", gotAlpha, wantAlpha)
	}
	if !reflect.DeepEqual(gotBeta, wantBeta) {
		t.Fatalf("Recent(beta) = %v, want %v", gotBeta, wantBeta)
	}
}

func TestTopicBuffers_TopicsSorted(t *testing.T) {
	tb := NewTopicBuffers(1)

	tb.Add(makeEnv("zeta", "z"))
	tb.Add(makeEnv("alpha", "a"))
	tb.Add(makeEnv("gamma", "g"))

	topics := tb.Topics()
	sorted := append([]string(nil), topics...)
	sort.Strings(sorted)

	if !reflect.DeepEqual(topics, sorted) {
		t.Fatalf("Topics() = %v, want sorted %v", topics, sorted)
	}
}

func TestTopicBuffers_RecentUnknownTopic(t *testing.T) {
	tb := NewTopicBuffers(2)

	// no additions for topic "unknown"
	got := tb.Recent("unknown", 10)
	if len(got) != 0 {
		t.Fatalf("Recent(unknown) = %v, want nil or empty slice", got)
	}
}
