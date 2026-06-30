package auditlog

import "time"

type Log struct {
	ID        string
	Actor     string
	Action    string
	Target    string
	Payload   string
	CreatedAt time.Time
}
