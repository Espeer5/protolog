/*******************************************************************************
*  internal/storage/writer.go
*
*  The storage writer is responsible for writing incoming logs received through 
*  the log collector into permanent storage, so that the logs may be queried
*  by the user through the GUI or other means.
*******************************************************************************/

package storage

/*******************************************************************************
*  IMPORTS
*******************************************************************************/

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

/*******************************************************************************
*  TYPES
*******************************************************************************/

type Writer struct {
	baseDir string

	mu    sync.Mutex
	files map[string]*os.File // topic -> file
}

/*******************************************************************************
*  FUNCTIONS
*******************************************************************************/

// NewWriter creates a storage writer that writes per-topic log files
// under baseDir (e.g. "data").
func NewWriter(baseDir string) (*Writer, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create baseDir: %w", err)
	}
	return &Writer{
		baseDir: baseDir,
		files:   make(map[string]*os.File),
	}, nil
}

// Close closes all open files.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var firstErr error
	for topic, f := range w.files {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close %s: %w", topic, err)
		}
	}
	return firstErr
}

// WriteEnvelope appends a single LogEnvelope to the per-topic file.
// Format: [4-byte big-endian length][protobuf bytes].
func (w *Writer) WriteEnvelope(env *logging.LogEnvelope) error {
	// Marshal protobuf
	data, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	topic := env.GetTopic()
	if topic == "" {
		topic = "untitled"
	}

	f, err := w.getFile(topic)
	if err != nil {
		return err
	}

	// Length prefix
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(data)))

	// Write length + data atomically with respect to other goroutines
	if _, err := f.Write(buf[:]); err != nil {
		return fmt.Errorf("write length: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	return nil
}

// getFile lazily opens/creates the file for a topic.
func (w *Writer) getFile(topic string) (*os.File, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if f, ok := w.files[topic]; ok {
		return f, nil
	}

	filename := filepath.Join(w.baseDir, fmt.Sprintf("%s.log", topic))
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open file for topic %q: %w", topic, err)
	}

	w.files[topic] = f
	return f, nil
}
