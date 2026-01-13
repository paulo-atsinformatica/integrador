package models

import "time"

// SyncPayload define o envelope de dados trafegado entre os nós
type SyncPayload struct {
	EventID     string                 `json:"event_id"`
	Table       string                 `json:"table"`
	Operation   string                 `json:"operation"`
	PKJSON      map[string]interface{} `json:"pk"`
	PayloadJSON map[string]interface{} `json:"data"`
	Origem      string                 `json:"source_node"`
	Timestamp   time.Time              `json:"timestamp"`
	RemoteAddr  string                 `json:"-"`
}
