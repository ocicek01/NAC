package portevent

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type claimRow struct {
	argCount int
}

func (r claimRow) Scan(dest ...any) error {
	if r.argCount != 1 {
		return fmt.Errorf("expected 1 arguments, got %d", r.argCount)
	}
	*(dest[0].(*string)) = IngestClaimed
	return nil
}

type claimDB struct {
	argCount int
}

func (db *claimDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (db *claimDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("unexpected query")
}

func (db *claimDB) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	db.argCount = len(args)
	return claimRow{argCount: len(args)}
}

func TestBeginIngestPassesEventIDToClaimQuery(t *testing.T) {
	db := &claimDB{}
	repository := NewPostgresRepository(db)
	status, err := repository.BeginIngest(context.Background(), "event-id")
	if err != nil {
		t.Fatal(err)
	}
	if status != IngestClaimed || db.argCount != 1 {
		t.Fatalf("status=%q argCount=%d", status, db.argCount)
	}
}
