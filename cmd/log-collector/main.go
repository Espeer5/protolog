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
	"encoding/json"
    "flag"
    "fmt"
    "log"
    "net/http"
    "strconv"
    "time"

    "github.com/gorilla/websocket"
    "github.com/pebbe/zmq4"
    "google.golang.org/protobuf/proto"

	"github.com/Espeer5/protolog/internal/config"
	"github.com/Espeer5/protolog/internal/memory"
	"github.com/Espeer5/protolog/internal/storage"
	"github.com/Espeer5/protolog/pkg/logproto/logging"
)

/*******************************************************************************
*  TYPES
*******************************************************************************/

type logDTO struct {
    Topic     string `json:"topic"`
    Timestamp string `json:"timestamp"`
    Level     string `json:"level"`
    Host      string `json:"host"`
    Service   string `json:"service"`
    Summary   string `json:"summary"`
    Type      string `json:"type"`
}


type client struct {
    topic string
    hub   *hub
    conn  *websocket.Conn
    send  chan logDTO
}

type hub struct {
    buffers    *memory.TopicBuffers
    register   chan *client
    unregister chan *client
    broadcast  chan *logging.LogEnvelope
    clients    map[*client]struct{}
}

/*******************************************************************************
*  FUNCTIONS
*******************************************************************************/

func newHub(buffers *memory.TopicBuffers) *hub {
    return &hub{
        buffers:    buffers,
        register:   make(chan *client),
        unregister: make(chan *client),
        broadcast:  make(chan *logging.LogEnvelope, 1024),
        clients:    make(map[*client]struct{}),
    }
}

func (h *hub) run() {
    for {
        select {
        case c := <-h.register:
            h.clients[c] = struct{}{}
        case c := <-h.unregister:
            if _, ok := h.clients[c]; ok {
                delete(h.clients, c)
                close(c.send)
                _ = c.conn.Close()
            }
        case env := <-h.broadcast:
            // fan out to clients that care about this topic
            for c := range h.clients {
                if c.topic != "" && c.topic != env.GetTopic() {
                    continue
                }
                dto := envToDTO(env)
                select {
                case c.send <- dto:
                default:
                    // client is too slow; drop message
                }
            }
        }
    }
}

func envToDTO(e *logging.LogEnvelope) logDTO {
    ts := ""
    if e.GetTimestamp() != nil {
        ts = e.GetTimestamp().AsTime().Format(time.RFC3339Nano)
    }
    return logDTO{
        Topic:     e.GetTopic(),
        Timestamp: ts,
        Level:     e.GetLevel().String(),
        Host:      e.GetHost(),
        Service:   e.GetService(),
        Summary:   e.GetSummary(),
        Type:      e.GetType(),
    }
}

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        // for now, allow all origins; tighten later if needed
        return true
    },
}

func (h *hub) serveWS(w http.ResponseWriter, r *http.Request) {
    topic := r.URL.Query().Get("topic") // empty means "all topics"

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("websocket upgrade error: %v", err)
        return
    }

    c := &client{
        topic: topic,
        hub:   h,
        conn:  conn,
        send:  make(chan logDTO, 256),
    }

    // send recent history first
    if topic != "" {
        recent := h.buffers.Recent(topic, 50)
        for _, e := range recent {
            c.send <- envToDTO(e)
        }
    }

    h.register <- c

    // writer goroutine
    go func() {
        for msg := range c.send {
            if err := c.conn.WriteJSON(msg); err != nil {
                break
            }
        }
        // when send channel closes, ensure we unregister
        h.unregister <- c
    }()

    // reader loop (just to detect close)
    go func() {
        defer func() {
            h.unregister <- c
        }()
        for {
            if _, _, err := c.conn.ReadMessage(); err != nil {
                break
            }
        }
    }()
}

func startHTTPServer(httpAddr string, buffers *memory.TopicBuffers, h *hub) {
    mux := http.NewServeMux()

    // GET /api/topics
    mux.HandleFunc("/api/topics", func(w http.ResponseWriter, r *http.Request) {
        topics := buffers.Topics()
        resp := map[string]any{
            "topics": topics,
        }
        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(resp); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
    })

	// Maintain a REST endpoint as well as the WS
    mux.HandleFunc("/api/logs/recent", func(w http.ResponseWriter, r *http.Request) {
        topic := r.URL.Query().Get("topic")
        if topic == "" {
            http.Error(w, "missing topic", http.StatusBadRequest)
            return
        }

        limitStr := r.URL.Query().Get("limit")
        limit := 100
        if limitStr != "" {
            if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
                limit = v
            }
        }

        envs := buffers.Recent(topic, limit)
        out := make([]logDTO, 0, len(envs))
        for _, e := range envs {
            out = append(out, envToDTO(e))
        }

        resp := map[string]any{
            "topic": topic,
            "logs":  out,
        }

        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(resp); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
    })

    // WebSocket endpoint for live logs
    mux.HandleFunc("/ws/logs", func(w http.ResponseWriter, r *http.Request) {
        h.serveWS(w, r)
    })

    // Static files (GUI) from ./ui/static
    fs := http.FileServer(http.Dir("ui/static"))
    mux.Handle("/", fs)

    go func() {
        log.Printf("Starting HTTP server on %s", httpAddr)
        if err := http.ListenAndServe(httpAddr, mux); err != nil {
            log.Fatalf("HTTP server error: %v", err)
        }
    }()
}


/*******************************************************************************
*  MAIN EXECUTABLE
*******************************************************************************/

func main() {
	addr := flag.String("addr", "tcp://localhost:5556",
		"ZMQ address of the log publisher")

	dataDir := flag.String("data-dir", config.DefaultDataDir(),
		"directory to store per-topic log files")

	bufferSize := flag.Int("buffer-size", config.DefaultBufferSize,
		"number of recent log messages to keep in memory per topic")

	httpAddr := flag.String("http-addr", ":8080",
        "HTTP listen address for API and GUI (e.g. :8080)")

	flag.Parse()

	log.Printf("Using data dir: %s", *dataDir)
	log.Printf("Ring buffer size: %d", *bufferSize)
	log.Printf("HTTP listen address: %s", *httpAddr)

	topicBuffers := memory.NewTopicBuffers(*bufferSize)

	// WebSocket hub
    h := newHub(topicBuffers)
    go h.run()

    // HTTP server (REST + static GUI + WebSockets)
    startHTTPServer(*httpAddr, topicBuffers, h)

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

		// Store in in-memory ring buffer for quick recent-access
		topicBuffers.Add(&env)

		h.broadcast <- &env

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
