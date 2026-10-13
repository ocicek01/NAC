package switchport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	porteventdomain "nac/internal/domain/portevent"
	"nac/internal/domain/switchasset"
	auditrepo "nac/internal/repository/auditlog"
	porteventrepo "nac/internal/repository/portevent"
	auditservice "nac/internal/service/auditlog"
	"nac/internal/snmp"
)

type observationState struct {
	Admin string   `json:"admin_status"`
	Oper  string   `json:"oper_status"`
	VLAN  int      `json:"vlan_id"`
	MACs  []string `json:"mac_addresses"`
}

type portClassification struct {
	Uplink bool
	Trunk  bool
	Mode   string
}

func shouldPublishMACDeltas(missing bool, classification portClassification) bool {
	if missing || classification.Uplink || classification.Trunk {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(classification.Mode), "trunk")
}

func mergeObservation(previous observationState, item snmp.PortObservation) observationState {
	next := previous
	next.Admin, next.Oper = item.AdminStatus, item.OperStatus
	if item.VLANID != nil {
		next.VLAN = *item.VLANID
	}
	if item.MACsKnown {
		next.MACs = uniqueNormalizedMACs(item.MACAddresses)
	}
	if next.MACs == nil {
		next.MACs = []string{}
	}
	slices.Sort(next.MACs)
	return next
}

func macDelta(previous, current []string) (learned, removed []string) {
	oldSet := make(map[string]struct{}, len(previous))
	newSet := make(map[string]struct{}, len(current))
	for _, mac := range previous {
		oldSet[mac] = struct{}{}
	}
	for _, mac := range current {
		newSet[mac] = struct{}{}
		if _, ok := oldSet[mac]; !ok {
			learned = append(learned, mac)
		}
	}
	for _, mac := range previous {
		if _, ok := newSet[mac]; !ok {
			removed = append(removed, mac)
		}
	}
	return learned, removed
}

func snapshotConflicts(updatedAt, pollStartedAt time.Time) bool {
	return !updatedAt.IsZero() && updatedAt.After(pollStartedAt)
}

// Observe persists a previously collected snapshot in a short transaction. SNMP
// I/O is deliberately completed before this method acquires a DB connection.
func (r *PostgresRepository) Observe(ctx context.Context, asset switchasset.Switch, snapshot snmp.PortSnapshot, started time.Time) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.Background())
	// Cross-process serialization is limited to the short persistence phase;
	// SNMP collection above never holds a DB session or transaction.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,73429))`, asset.ID); err != nil {
		return 0, err
	}
	events := porteventrepo.NewPostgresRepository(tx)
	audit := auditservice.NewService(auditrepo.NewPostgresRepository(tx))
	changed := 0
	for _, item := range snapshot.Ports {
		var previous observationState
		var classification portClassification
		var raw []byte
		var updated time.Time
		err = tx.QueryRow(ctx, `SELECT admin_status, oper_status, vlan_id, mac_addresses, updated_at,
			is_uplink, is_trunk, port_mode
		 FROM switch_ports WHERE switch_id=$1 AND if_index=$2 FOR UPDATE`, asset.ID, item.IfIndex).
			Scan(&previous.Admin, &previous.Oper, &previous.VLAN, &raw, &updated,
				&classification.Uplink, &classification.Trunk, &classification.Mode)
		missing := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !missing {
			return 0, err
		}
		if len(raw) > 0 {
			if err = json.Unmarshal(raw, &previous.MACs); err != nil {
				return 0, err
			}
		}
		previous.MACs = uniqueNormalizedMACs(previous.MACs)
		slices.Sort(previous.MACs)
		// A trap or refresh committed after this poll started. Applying any part
		// of the stale snapshot would regress VLAN/MAC state or create false
		// deltas, so leave the entire port for the next poll.
		if !missing && snapshotConflicts(updated, started) {
			continue
		}
		next := mergeObservation(previous, item)
		if !missing && previous.Admin == next.Admin && previous.Oper == next.Oper && previous.VLAN == next.VLAN && slices.Equal(previous.MACs, next.MACs) {
			if _, err = tx.Exec(ctx, `UPDATE switch_ports SET last_discovered_at=$3 WHERE switch_id=$1 AND if_index=$2`, asset.ID, item.IfIndex, time.Now().UTC()); err != nil {
				return 0, err
			}
			continue
		}
		now := time.Now().UTC()
		macJSON, _ := json.Marshal(next.MACs)
		_, err = tx.Exec(ctx, `INSERT INTO switch_ports
		 (id, switch_id, if_index, port_index, interface_name, interface_description, interface_type, is_physical, admin_status, oper_status,
		 status, vlan_id, mac_count, mac_addresses, last_changed_at, last_discovered_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$3,$4,$5,$6,true,$7,$8,CASE WHEN $7='down' THEN 'disabled' WHEN $8='up' THEN 'up' ELSE 'down' END,$9,$10,$11,$12,$12,$12,$12)
		 ON CONFLICT (switch_id,if_index) DO UPDATE SET
		 interface_name=COALESCE(NULLIF(EXCLUDED.interface_name,''),switch_ports.interface_name),
		 interface_description=COALESCE(NULLIF(EXCLUDED.interface_description,''),switch_ports.interface_description),
		 interface_type=EXCLUDED.interface_type,is_physical=true,
		 admin_status=EXCLUDED.admin_status,oper_status=EXCLUDED.oper_status,status=EXCLUDED.status,
		 vlan_id=EXCLUDED.vlan_id,mac_count=EXCLUDED.mac_count,mac_addresses=EXCLUDED.mac_addresses,
		 last_changed_at=EXCLUDED.last_changed_at,last_discovered_at=EXCLUDED.last_discovered_at,updated_at=EXCLUDED.updated_at`,
			uuid.NewString(), asset.ID, item.IfIndex, item.Name, item.Description, fmt.Sprintf("ifType:%d", item.IfType), next.Admin, next.Oper, next.VLAN, len(next.MACs), macJSON, now)
		if err != nil {
			return 0, err
		}
		event, err := events.Insert(ctx, porteventdomain.Event{ID: uuid.NewString(), SwitchID: asset.ID, SwitchName: asset.Name,
			ManagementIP: asset.ManagementIP, IfIndex: item.IfIndex, InterfaceName: item.Name, InterfaceDescription: item.Description,
			AdminStatus: next.Admin, OperStatus: next.Oper, EventType: "port_snapshot_changed", EventSource: "port-observer",
			Metadata: map[string]any{"previous": previous, "current": next, "initial": missing}, ObservedAt: now, CreatedAt: now})
		if err != nil {
			return 0, err
		}
		if err = audit.Record(ctx, "port_snapshot_changed", "success", "port_event", event.ID, asset.ID, "", event.Metadata); err != nil {
			return 0, err
		}
		if item.MACsKnown && shouldPublishMACDeltas(missing, classification) {
			learned, removed := macDelta(previous.MACs, next.MACs)
			for _, delta := range []struct {
				typeName string
				values   []string
			}{{"mac_learned", learned}, {"mac_removed", removed}} {
				for _, mac := range delta.values {
					outboxEvent := porteventdomain.Event{
						ID: uuid.NewString(), SwitchID: asset.ID, SwitchName: asset.Name, ManagementIP: asset.ManagementIP,
						IfIndex: item.IfIndex, InterfaceName: item.Name, InterfaceDescription: item.Description,
						AdminStatus: next.Admin, OperStatus: next.Oper, EventType: delta.typeName,
						EventSource: "port-observer", MACAddress: mac,
						Metadata: map[string]any{"snapshot_event_id": event.ID}, ObservedAt: now, CreatedAt: now,
					}
					payload, marshalErr := json.Marshal(outboxEvent)
					if marshalErr != nil {
						return 0, marshalErr
					}
					if _, err = tx.Exec(ctx, `INSERT INTO port_observer_outbox (id,payload) VALUES ($1,$2::jsonb) ON CONFLICT (id) DO NOTHING`, outboxEvent.ID, payload); err != nil {
						return 0, err
					}
				}
			}
		}
		changed++
	}
	return changed, tx.Commit(ctx)
}

func (r *PostgresRepository) PendingPortEvents(ctx context.Context, limit int) ([]porteventdomain.Event, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := r.pool.Query(ctx, `WITH due AS (
		SELECT id FROM port_observer_outbox
		WHERE delivered_at IS NULL AND failed_at IS NULL AND attempts < 5 AND available_at<=NOW()
		ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $1
	) UPDATE port_observer_outbox o SET available_at=NOW()+INTERVAL '1 minute'
	FROM due WHERE o.id=due.id RETURNING o.payload`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]porteventdomain.Event, 0, limit)
	for rows.Next() {
		var raw []byte
		var event porteventdomain.Event
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &event); err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) MarkPortEventDelivered(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE port_observer_outbox SET delivered_at=NOW(),last_error='' WHERE id=$1`, id)
	return err
}

func (r *PostgresRepository) RetryPortEvent(ctx context.Context, id, message string, permanent bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE port_observer_outbox SET attempts=attempts+1,last_error=$2,
		failed_at=CASE WHEN $3 OR attempts+1>=5 THEN NOW() ELSE failed_at END,
		available_at=CASE WHEN $3 OR attempts+1>=5 THEN available_at ELSE NOW()+(LEAST(attempts+1,6)*INTERVAL '10 seconds') END
		WHERE id=$1 AND delivered_at IS NULL AND failed_at IS NULL`, id, message, permanent)
	return err
}
