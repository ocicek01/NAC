package portevent

import (
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	created := time.Date(2026, 10, 13, 8, 30, 45, 123456000, time.UTC)
	id := "7727e1b3-a17a-4576-9b63-345f508619d4"
	cursor := encodeCursor(created, id)
	parsed, parsedID, err := decodeCursor(cursor)
	if err != nil || !parsed.Equal(created) || parsedID != id {
		t.Fatalf("cursor=%q parsed=%v id=%q err=%v", cursor, parsed, parsedID, err)
	}
}

func TestCursorRejectsMalformedValue(t *testing.T) {
	if _, _, err := decodeCursor("not-a-cursor"); err == nil {
		t.Fatal("malformed cursor accepted")
	}
}
