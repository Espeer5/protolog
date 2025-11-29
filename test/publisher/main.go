package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/pebbe/zmq4"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Espeer5/protolog/pkg/logproto/demo"
	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

type topicSpec struct {
	name     string
	level    logging.LogLevel
	typeName string
}

func main() {
	rand.Seed(time.Now().UnixNano())

	endpoint := "tcp://127.0.0.1:5556" // or make it a flag

	pub, err := zmq4.NewSocket(zmq4.PUB)
	if err != nil {
		log.Fatalf("failed to create PUB socket: %v", err)
	}
	defer pub.Close()

	if err := pub.Connect(endpoint); err != nil {
		log.Fatalf("failed to connect PUB socket: %v", err)
	}

	selfHost, _ := os.Hostname()
	pid := os.Getpid()

	// Simulated hosts & services
	hosts := []string{
		selfHost,
		"node-a.internal",
		"node-b.internal",
	}

	services := []string{
		"api-gateway",
		"worker",
		"metrics-agent",
		"alert-manager",
		"audit-service",
	}

	// Topic mapping: topic -> (level, typeName)
	topics := []topicSpec{
		{name: "demo", level: logging.LogLevel_LOG_LEVEL_INFO, typeName: "demo.Message"},
		{name: "metrics", level: logging.LogLevel_LOG_LEVEL_DEBUG, typeName: "demo.Metric"},
		{name: "alerts", level: logging.LogLevel_LOG_LEVEL_WARN, typeName: "demo.Alert"},
		{name: "audit", level: logging.LogLevel_LOG_LEVEL_INFO, typeName: "demo.AuditEvent"},
	}

	log.Println("Test publisher started on tcp://*:5556")
	time.Sleep(500 * time.Millisecond) // allow SUB to connect

	seq := 0

	for {
		for _, t := range topics {
			host := hosts[rand.Intn(len(hosts))]
			service := services[rand.Intn(len(services))]

			payload, summary := buildPayload(t, seq, host, service)

			env := &logging.LogEnvelope{
				Topic:     t.name,
				Timestamp: timestamppb.Now(),
				Level:     t.level,
				Host:      host,
				Service:   service,
				Pid:       int32(pid),

				Type:    t.typeName,
				Payload: payload,

				Summary: summary,
			}

			data, err := proto.Marshal(env)
			if err != nil {
				log.Printf("marshal envelope error for topic %q: %v\n", t.name, err)
				continue
			}

			if _, err := pub.SendBytes(data, 0); err != nil {
				log.Printf("send error for topic %q: %v\n", t.name, err)
				continue
			}

			log.Printf("Sent %s [%s] host=%s service=%s seq=%d\n",
				t.name, t.typeName, host, service, seq)

			seq++
		}

		time.Sleep(1 * time.Second)
	}
}

func buildPayload(t topicSpec, seq int, host, service string) ([]byte, string) {
	switch t.typeName {
	case "demo.Message":
		msg := &demo.Message{
			Text:  fmt.Sprintf("Hello from topic %q (service=%s)", t.name, service),
			Count: int32(seq),
		}
		b, _ := proto.Marshal(msg)
		summary := fmt.Sprintf("DEMO msg #%d on %s (%s)", seq, t.name, service)
		return b, summary

	case "demo.Metric":
		metric := &demo.Metric{
			Name:      "cpu_usage",
			Value:     10 + float64(seq%90), // 10–99
			Unit:      "percent",
			Host:      host,
			Subsystem: service,
		}
		b, _ := proto.Marshal(metric)
		summary := fmt.Sprintf("METRIC cpu_usage=%.1f%% host=%s svc=%s", metric.Value, host, service)
		return b, summary

	case "demo.Alert":
		severities := []string{"INFO", "WARN", "CRITICAL"}
		severity := severities[seq%len(severities)]

		alert := &demo.Alert{
			Id:          fmt.Sprintf("ALERT-%04d", seq),
			Severity:    severity,
			Description: fmt.Sprintf("Synthetic alert #%d from %s", seq, service),
			Source:      service,
			IncidentId:  int64(1000 + seq),
		}
		b, _ := proto.Marshal(alert)
		summary := fmt.Sprintf("ALERT %s id=%s src=%s", severity, alert.Id, service)
		return b, summary

	case "demo.AuditEvent":
		actors := []string{"alice", "bob", "carol", "system"}
		actions := []string{"LOGIN", "LOGOUT", "CREATE", "DELETE"}
		resources := []string{"project/alpha", "project/beta", "user/settings"}

		audit := &demo.AuditEvent{
			Actor:    actors[seq%len(actors)],
			Action:   actions[seq%len(actions)],
			Resource: resources[seq%len(resources)],
			Success: map[bool]int64{true: 1, false: 0}[seq%2 == 0],
			Details:  fmt.Sprintf("Audit seq=%d from %s", seq, service),
		}
		b, _ := proto.Marshal(audit)
		summary := fmt.Sprintf("AUDIT actor=%s action=%s res=%s", audit.Actor, audit.Action, audit.Resource)
		return b, summary

	default:
		// Fallback: no payload
		return nil, fmt.Sprintf("Unknown type %s for topic %s", t.typeName, t.name)
	}
}
