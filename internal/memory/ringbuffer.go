/*******************************************************************************
*  internal/memory/ringbuffer.go
*
*  protolog maintains an in-proc memory in the form of ringbuffers of the last
*  N messages of any given topic. This allows rapid querying of recent data by
*  the GUI/user.
*******************************************************************************/

package memory

/*******************************************************************************
*  IMPORTS
*******************************************************************************/

import (
	"sort"
	"sync"

	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

/*******************************************************************************
*  TYPES
*******************************************************************************/

// RingBuffer holds the last N log envelopes for a single topic.
type RingBuffer struct {
	mu    sync.RWMutex
	buf   []*logging.LogEnvelope
	size  int
	start int // index of oldest element
	count int // how many valid elements
}

/*******************************************************************************
*  FUNCITONS
*******************************************************************************/

func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 1
	}
	return &RingBuffer{
		buf:  make([]*logging.LogEnvelope, size),
		size: size,
	}
}

// Add appends an envelope, overwriting the oldest if full.
func (r *RingBuffer) Add(env *logging.LogEnvelope) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count < r.size {
		idx := (r.start + r.count) % r.size
		r.buf[idx] = env
		r.count++
	} else {
		// overwrite oldest
		r.buf[r.start] = env
		r.start = (r.start + 1) % r.size
	}
}

// Recent returns up to max most recent envelopes, newest-first.
func (r *RingBuffer) Recent(max int) []*logging.LogEnvelope {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.count == 0 {
		return nil
	}

	if max <= 0 || max > r.count {
		max = r.count
	}

	out := make([]*logging.LogEnvelope, 0, max)
	// iterate from newest backwards
	for i := 0; i < max; i++ {
		idx := (r.start + r.count - 1 - i + r.size) % r.size
		out = append(out, r.buf[idx])
	}
	return out
}

// TopicBuffers manages ring buffers per topic.
type TopicBuffers struct {
	mu     sync.RWMutex
	size   int
	topics map[string]*RingBuffer
}

func NewTopicBuffers(size int) *TopicBuffers {
	if size <= 0 {
		size = 1
	}
	return &TopicBuffers{
		size:   size,
		topics: make(map[string]*RingBuffer),
	}
}

func (tb *TopicBuffers) getOrCreate(topic string) *RingBuffer {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if topic == "" {
		topic = "untitled"
	}

	if rb, ok := tb.topics[topic]; ok {
		return rb
	}
	rb := NewRingBuffer(tb.size)
	tb.topics[topic] = rb
	return rb
}

// Add stores the envelope in the appropriate topic buffer.
func (tb *TopicBuffers) Add(env *logging.LogEnvelope) {
	topic := env.GetTopic()
	rb := tb.getOrCreate(topic)
	rb.Add(env)
}

// Recent returns up to max most recent envelopes for a topic, newest-first.
func (tb *TopicBuffers) Recent(topic string, max int) []*logging.LogEnvelope {
	tb.mu.RLock()
	rb, ok := tb.topics[topic]
	tb.mu.RUnlock()
	if !ok {
		return nil
	}
	return rb.Recent(max)
}

// Topics returns the list of topics currently seen in memory.
func (tb *TopicBuffers) Topics() []string {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	names := make([]string, 0, len(tb.topics))
	for t := range tb.topics {
		names = append(names, t)
	}
	sort.Strings(names)
	return names
}
