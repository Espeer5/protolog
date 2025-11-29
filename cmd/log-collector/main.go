/*******************************************************************************
*  cmd/log-collector/main.go
*
*  The log-collector is the backend of the protobuf logging facility. It
*  operates a SUB socket which receives all logged messages and handles all
*  functions of the logging engine including filtering, ordering, and writing.
*  It streams data to and receives commands from the GUI application.
*******************************************************************************/

package main

/*******************************************************************************
*  IMPORTS
*******************************************************************************/

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/pebbe/zmq4"
	"google.golang.org/protobuf/proto"

	"github.com/Espeer5/protolog/internal/config"
	"github.com/Espeer5/protolog/internal/storage"
	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

/*******************************************************************************
*  MAIN EXECUTABLE
*******************************************************************************/

func main() {
	addr := flag.String("addr", "tcp://localhost:5556", "ZMQ address of the log publisher")
	dataDir := flag.String("data-dir", config.DefaultDataDir(), "directory to store per-topic log files")
	flag.Parse()

	log.Printf("Using data dir: %s", *dataDir)

	writer, err := storage.NewWriter(*dataDir)
	if err != nil {
		log.Fatalf("failed to init storage: %v", err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			log.Printf("error closing storage: %v", err)
		}
	}()

	sub, err := zmq4.NewSocket(zmq4.SUB)
	if err != nil {
		log.Fatalf("failed to create SUB socket: %v", err)
	}
	defer sub.Close()

	if err := sub.SetSubscribe(""); err != nil {
		log.Fatalf("failed to set SUBSCRIBE: %v", err)
	}

	log.Printf("Connecting SUB to %s ...", *addr)
	if err := sub.Connect(*addr); err != nil {
		log.Fatalf("failed to connect SUB socket: %v", err)
	}
	log.Printf("Connected. Waiting for log envelopes...")

	for {
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

		if err := writer.WriteEnvelope(&env); err != nil {
			log.Printf("failed to write envelope to storage: %v", err)
		}

		t := time.Unix(0, 0)
		if ts := env.GetTimestamp(); ts != nil {
			t = ts.AsTime()
		}

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
