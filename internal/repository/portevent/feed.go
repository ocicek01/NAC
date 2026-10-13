package portevent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	domain "nac/internal/domain/portevent"
)

func (r *PostgresRepository) Feed(ctx context.Context, switchID, after string) (domain.Feed, error) {
	feed := domain.Feed{Cursor: after, Events: []domain.Event{}}
	if after == "" {
		rows, err := r.pool.Query(ctx, `SELECT created_at,id FROM port_events
			WHERE switch_id=$1 AND event_source='port-observer' AND event_type='port_snapshot_changed'
			ORDER BY created_at DESC,id DESC LIMIT 1`, switchID)
		if err != nil {
			return feed, err
		}
		defer rows.Close()
		if rows.Next() {
			var created time.Time
			var id string
			if err = rows.Scan(&created, &id); err != nil {
				return feed, err
			}
			feed.Cursor = encodeCursor(created, id)
		}
		return feed, rows.Err()
	}

	created, id, err := decodeCursor(after)
	if err != nil {
		return feed, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id,if_index,interface_name,admin_status,oper_status,metadata,observed_at,created_at
		FROM port_events
		WHERE switch_id=$1 AND event_source='port-observer' AND event_type='port_snapshot_changed' AND (created_at,id)>($2,$3::uuid)
		ORDER BY created_at,id LIMIT 250`, switchID, created, id)
	if err != nil {
		return feed, err
	}
	defer rows.Close()
	for rows.Next() {
		item := domain.Event{SwitchID: switchID, EventSource: "port-observer"}
		var raw []byte
		if err = rows.Scan(&item.ID, &item.IfIndex, &item.InterfaceName, &item.AdminStatus, &item.OperStatus, &raw, &item.ObservedAt, &item.CreatedAt); err != nil {
			return feed, err
		}
		if err = json.Unmarshal(raw, &item.Metadata); err != nil {
			return feed, err
		}
		feed.Cursor = encodeCursor(item.CreatedAt, item.ID)
		feed.Events = append(feed.Events, item)
	}
	return feed, rows.Err()
}

func encodeCursor(created time.Time, id string) string {
	return strconv.FormatInt(created.UTC().UnixNano(), 10) + "_" + id
}

func decodeCursor(value string) (time.Time, string, error) {
	separator := strings.IndexByte(value, '_')
	if separator <= 0 || separator == len(value)-1 {
		return time.Time{}, "", fmt.Errorf("invalid event cursor")
	}
	nanos, err := strconv.ParseInt(value[:separator], 10, 64)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid event cursor")
	}
	id := value[separator+1:]
	if _, err = uuid.Parse(id); err != nil {
		return time.Time{}, "", fmt.Errorf("invalid event cursor")
	}
	return time.Unix(0, nanos).UTC(), id, nil
}
