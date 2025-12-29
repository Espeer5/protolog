package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Espeer5/protolog/pkg/logproto/logging"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func OpenSQLite(path string) (*sql.DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA temp_store=MEMORY;
	`); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func InitSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS logs (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		event_ts_ms    INTEGER NOT NULL,
		ingest_ts_ms   INTEGER NOT NULL,
		topic          TEXT NOT NULL,
		level          INTEGER NOT NULL,
		host           TEXT,
		service        TEXT,
		pid            INTEGER,
		type           TEXT,
		summary        TEXT,
		session_id     TEXT,
		correlation_id TEXT,
		payload        BLOB
	);

	CREATE INDEX IF NOT EXISTS idx_logs_service_event
		ON logs(service, event_ts_ms, id);

	CREATE INDEX IF NOT EXISTS idx_logs_host_event
		ON logs(host, event_ts_ms, id);

	CREATE INDEX IF NOT EXISTS idx_logs_type_event
		ON logs(type, event_ts_ms, id);

	CREATE INDEX IF NOT EXISTS idx_logs_level_event
		ON logs(level, event_ts_ms, id);
	`
	_, err := db.Exec(schema)
	return err
}

func tsToMillis(ts *timestamppb.Timestamp) int64 {
	if ts == nil {
		return 0
	}
	return ts.Seconds*1000 + int64(ts.Nanos)/1e6
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullBlob(b []byte) any {
	if len(b) == 0 {
		return nil // store SQL NULL when payload is "None"
	}
	return b
}

// InsertLog stores the envelope metadata + the raw payload bytes (nullable).
func InsertLog(db *sql.DB, env *logging.LogEnvelope) error {
	_, err := db.Exec(`
		INSERT INTO logs (
			event_ts_ms,
			ingest_ts_ms,
			topic,
			level,
			host,
			service,
			pid,
			type,
			summary,
			session_id,
			correlation_id,
			payload
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		tsToMillis(env.Timestamp),
		time.Now().UnixMilli(),
		env.Topic,
		int(env.Level),
		nullString(env.Host),
		nullString(env.Service),
		env.Pid,
		nullString(env.Type),
		nullString(env.Summary),
		nullString(env.SessionId),
		nullString(env.CorrelationId),
		nullBlob(env.Payload),
	)
	return err
}

type LogRow struct {
	ID        int64
	EventTSMs int64
	Topic     string
	Service   sql.NullString
	Level     int
	Summary   sql.NullString
	Type      sql.NullString
	Host      sql.NullString
	Pid       sql.NullInt64
	Payload   []byte // nil if NULL
}

func QueryLogs(db *sql.DB,
	startMs, endMs int64,
	topic, service string,
	cursorTS, cursorID int64,
	limit int,
) ([]LogRow, error) {

	rows, err := db.Query(`
		SELECT
			id, event_ts_ms,
			topic, service, level,
			summary, type, host, pid,
			payload
		FROM logs
		WHERE event_ts_ms >= ?
		  AND event_ts_ms < ?
		  AND (? = '' OR topic = ?)
		  AND (? = '' OR service = ?)
		  AND (
		    ? = 0 OR
		    event_ts_ms > ? OR (event_ts_ms = ? AND id > ?)
		  )
		ORDER BY event_ts_ms ASC, id ASC
		LIMIT ?
	`,
		startMs, endMs,
		topic, topic,
		service, service,
		cursorTS,
		cursorTS, cursorTS, cursorID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LogRow, 0, limit)
	for rows.Next() {
		var r LogRow
		var payload sql.RawBytes

		if err := rows.Scan(
			&r.ID, &r.EventTSMs,
			&r.Topic, &r.Service, &r.Level,
			&r.Summary, &r.Type, &r.Host, &r.Pid,
			&payload,
		); err != nil {
			return nil, err
		}

		if payload != nil {
			r.Payload = append([]byte(nil), payload...) // copy
		} else {
			r.Payload = nil
		}

		out = append(out, r)
	}
	return out, rows.Err()
}

// QueryLogsMulti queries by time range + optional multi-value filters.
// Cursor is (event_ts_ms, id) for stable paging.
func QueryLogsMulti(
	db *sql.DB,
	startMs, endMs int64,
	topics []string,
	services []string,
	hosts []string,
	levels []int,   // optional
	types []string, // optional
	cursorTS, cursorID int64,
	limit int,
) ([]LogRow, error) {
	where := []string{
		"event_ts_ms >= ?",
		"event_ts_ms < ?",
	}
	args := []any{startMs, endMs}

	// helper to build "col IN (?, ?, ?)"
	addInStrings := func(col string, vals []string) {
		if len(vals) == 0 {
			return
		}
		ph := make([]string, 0, len(vals))
		for range vals {
			ph = append(ph, "?")
		}
		where = append(where, col+" IN ("+strings.Join(ph, ",")+")")
		for _, v := range vals {
			args = append(args, v)
		}
	}
	addInInts := func(col string, vals []int) {
		if len(vals) == 0 {
			return
		}
		ph := make([]string, 0, len(vals))
		for range vals {
			ph = append(ph, "?")
		}
		where = append(where, col+" IN ("+strings.Join(ph, ",")+")")
		for _, v := range vals {
			args = append(args, v)
		}
	}

	addInStrings("topic", topics)
	addInStrings("service", services)
	addInStrings("host", hosts)
	addInStrings("type", types)
	addInInts("level", levels)

	// cursor paging
	if cursorTS != 0 {
		where = append(where, "(event_ts_ms > ? OR (event_ts_ms = ? AND id > ?))")
		args = append(args, cursorTS, cursorTS, cursorID)
	}

	args = append(args, limit)

	q := `
SELECT
  id, event_ts_ms,
  topic, service, level,
  summary, type, host, pid,
  payload
FROM logs
WHERE ` + strings.Join(where, "\n  AND ") + `
ORDER BY event_ts_ms ASC, id ASC
LIMIT ?`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LogRow, 0, limit)
	for rows.Next() {
		var r LogRow
		var payload sql.RawBytes
		if err := rows.Scan(
			&r.ID, &r.EventTSMs,
			&r.Topic, &r.Service, &r.Level,
			&r.Summary, &r.Type, &r.Host, &r.Pid,
			&payload,
		); err != nil {
			return nil, err
		}
		if payload != nil {
			r.Payload = append([]byte(nil), payload...)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

