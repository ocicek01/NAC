package radiusevent

import (
	"context"

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
	`

	_, err := r.pool.Exec(ctx, query,
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
	)
	if err != nil {
		return domain.Event{}, err
	}

	return event, nil
}
