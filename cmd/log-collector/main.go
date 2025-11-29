// cmd/log-collector/main.go

package main


import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/pebbe/zmq4"
	"google.golang.org/protobuf/proto"

	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

func main() {
	addr := flag.String("addr", "tcp://localhost:5556", "ZMQ address of the log publisher (e.g. tcp://localhost:5556 or ipc:///tmp/logs.sock)")
	flag.Parse()

	// Create SUB socket
	sub, err := zmq4.NewSocket(zmq4.SUB)
	if err != nil {
		log.Fatalf("failed to create SUB socket: %v", err)
	}
	defer sub.Close()

	// We're sending a single frame with protobuf bytes only,
	// so we SUBSCRIBE to everything and filter by envelope.Topic in Go.
	if err := sub.SetSubscribe(""); err != nil {
		log.Fatalf("failed to set SUBSCRIBE: %v", err)
	}

	log.Printf("Connecting SUB to %s ...", *addr)
	if err := sub.Connect(*addr); err != nil {
		log.Fatalf("failed to connect SUB socket: %v", err)
	}
	log.Printf("Connected. Waiting for log envelopes...")

	for {
		// Single-frame receive (just the serialized LogEnvelope)
		data, err := sub.RecvBytes(0)
		if err != nil {
			log.Printf("recv error: %v", err)
			continue
		}

		var env logging.LogEnvelope
		if err := proto.Unmarshal(data, &env); err != nil {
			log.Printf("failed to unmarshal LogEnvelope: %v", err)
			continue
		}

		// Convert timestamp safely
		t := time.Unix(0, 0)
		if ts := env.GetTimestamp(); ts != nil {
			t = ts.AsTime()
		}

		// Simple human-readable line for now
		fmt.Printf("[%s] topic=%q level=%s host=%s service=%s pid=%d type=%s\n  summary=%s\n\n",
			t.Format(time.RFC3339Nano),
			env.GetTopic(),
			env.GetLevel().String(),
			env.GetHost(),
			env.GetService(),
			env.GetPid(),
			env.GetType(),
			env.GetSummary(),
		)
	}
}
