package radiusevent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/radiusevent"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, event domain.Event) (domain.Event, error) {
	query := `
		INSERT INTO radius_events (
			id,
			event_type,
			username,
			mac_address,
			hostname,
			vendor_class,
			nas_ip_address,
			nas_identifier,
			nas_port,
			nas_port_id,
			nas_port_type,
			called_station_id,
			calling_station_id,
			acct_status_type,
			acct_session_id,
			framed_ip_address,
			session_time,
			terminate_cause,
			decision,
			policy_action,
			policy_reason,
			reply_message,
			vlan_id,
			reply_attributes,
			control_attributes,
			created_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, NULLIF($7, '')::inet, $8, $9, $10,
			$11, $12, $13, $14, $15, NULLIF($16, '')::inet, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26
		)
		ON CONFLICT (id) DO NOTHING
		RETURNING id
	`

	var insertedID string
	err := r.pool.QueryRow(ctx, query,
		event.ID,
		event.EventType,
		event.Username,
		event.MACAddress,
		event.Hostname,
		event.VendorClass,
		event.NASIPAddress,
		event.NASIdentifier,
		event.NASPort,
		event.NASPortID,
		event.NASPortType,
		event.CalledStationID,
		event.CallingStationID,
		event.AcctStatusType,
		event.AcctSessionID,
		event.FramedIPAddress,
		event.SessionTime,
		event.TerminateCause,
		event.Decision,
		event.PolicyAction,
		event.PolicyReason,
		event.ReplyMessage,
		event.VLANID,
		event.ReplyAttributes,
		event.ControlAttributes,
		event.CreatedAt,
	).Scan(&insertedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Event{}, nil
		}
		return domain.Event{}, err
	}

	return event, nil
}

func (r *PostgresRepository) ClaimAccounting(ctx context.Context, event domain.Event, claimedAt, staleBefore time.Time) (domain.ClaimResult, error) {
	query := `
		WITH claimed AS (
			INSERT INTO radius_events (
				id, event_type, username, mac_address, hostname, vendor_class,
				nas_ip_address, nas_identifier, nas_port, nas_port_id, nas_port_type,
				called_station_id, calling_station_id, acct_status_type, acct_session_id,
				framed_ip_address, session_time, terminate_cause, decision, policy_action,
				policy_reason, reply_message, vlan_id, reply_attributes, control_attributes,
				created_at, received_at, processing_status, processing_started_at,
				processed_at, processing_error
			)
			VALUES (
				$1, $2, $3, $4, $5, $6, NULLIF($7, '')::inet, $8, $9, $10,
				$11, $12, $13, $14, $15, NULLIF($16, '')::inet, $17, $18, $19, $20,
				$21, $22, $23, $24, $25, $26, $27, 'processing', $27, NULL, ''
			)
			ON CONFLICT (id) DO UPDATE
			SET processing_status = 'processing',
				processing_started_at = EXCLUDED.processing_started_at,
				processed_at = NULL,
				processing_error = ''
			WHERE radius_events.processing_status = 'failed'
			   OR (
				radius_events.processing_status = 'processing'
				AND radius_events.processing_started_at < $28
			   )
			RETURNING processing_status
		)
		SELECT
			EXISTS(SELECT 1 FROM claimed),
			COALESCE(
				(SELECT processing_status FROM claimed),
				(SELECT processing_status FROM radius_events WHERE id = $1),
				'processing'
			)
	`

	var claimed bool
	var status string
	err := r.pool.QueryRow(ctx, query,
		event.ID,
		event.EventType,
		event.Username,
		event.MACAddress,
		event.Hostname,
		event.VendorClass,
		event.NASIPAddress,
		event.NASIdentifier,
		event.NASPort,
		event.NASPortID,
		event.NASPortType,
		event.CalledStationID,
		event.CallingStationID,
		event.AcctStatusType,
		event.AcctSessionID,
		event.FramedIPAddress,
		event.SessionTime,
		event.TerminateCause,
		event.Decision,
		event.PolicyAction,
		event.PolicyReason,
		event.ReplyMessage,
		event.VLANID,
		event.ReplyAttributes,
		event.ControlAttributes,
		event.CreatedAt,
		claimedAt,
		staleBefore,
	).Scan(&claimed, &status)
	if err != nil {
		return "", err
	}
	if claimed {
		return domain.Claimed, nil
	}
	if status == "processed" {
		return domain.Duplicate, nil
	}
	return domain.Busy, nil
}

func (r *PostgresRepository) CompleteAccounting(ctx context.Context, eventID string, claimedAt, completedAt time.Time) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE radius_events
		SET processing_status = 'processed',
			processed_at = $3,
			processing_error = ''
		WHERE id = $1
		  AND processing_status = 'processing'
		  AND processing_started_at = $2
	`, eventID, claimedAt, completedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("radius accounting event %s is not processing", eventID)
	}
	return nil
}

func (r *PostgresRepository) FailAccounting(ctx context.Context, eventID, processingError string, claimedAt, failedAt time.Time) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE radius_events
		SET processing_status = 'failed',
			processed_at = $3,
			processing_error = LEFT($4, 1000)
		WHERE id = $1
		  AND processing_status = 'processing'
		  AND processing_started_at = $2
	`, eventID, claimedAt, failedAt, processingError)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("radius accounting event %s is not processing", eventID)
	}
	return nil
}
