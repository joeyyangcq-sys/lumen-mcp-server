package audit

import "time"

type Event struct {
	At           time.Time
	Actor        string
	ClientID     string
	Tool         string
	ResourceKind string
	ResourceID   string
	Result       string
	TraceID      string
	Message      string
}
