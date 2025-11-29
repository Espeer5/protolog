package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/pebbe/zmq4"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

func main() {
	pub, err := zmq4.NewSocket(zmq4.PUB)
	if err != nil {
		log.Fatalf("failed to create PUB socket: %v", err)
	}
	defer pub.Close()

	if err := pub.Bind("tcp://*:5556"); err != nil {
		log.Fatalf("failed to bind PUB socket: %v", err)
	}

	host, _ := os.Hostname()
	pid := os.Getpid()

	// Give SUB time to connect
	time.Sleep(500 * time.Millisecond)

	i := 0
	for {
		env := &logging.LogEnvelope{
			Topic:     "demo",
			Timestamp: timestamppb.Now(),
			Level:     logging.LogLevel_LOG_LEVEL_INFO,
			Host:      host,
			Service:   "test-publisher",
			Pid:       int32(pid),
			Type:      "demo.Message",
			Payload:   []byte("dummy payload"),
			Summary:   "Hello from test publisher, count=" + strconv.Itoa(i),
		}

		data, err := proto.Marshal(env)
		if err != nil {
			log.Printf("marshal error: %v\n", err)
			continue
		}

		if _, err := pub.SendBytes(data, 0); err != nil {
			log.Printf("send error: %v\n", err)
			continue
		}

		log.Printf("Sent log envelope %d\n", i)
		i++

		time.Sleep(1 * time.Second)
	}
}
