package dhcpevent

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/dhcpevent"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) FindRecentByTransaction(ctx context.Context, macAddress, messageType, transactionID string, since time.Time) (*domain.Event, error) {
	query := `
		SELECT
			id,
			mac_address,
			COALESCE(transaction_id, ''),
			COALESCE(client_ip::text, ''),
			COALESCE(your_ip::text, ''),
			COALESCE(requested_ip::text, ''),
			COALESCE(hostname, ''),
			COALESCE(vendor_class, ''),
			COALESCE(option82_raw, ''),
			COALESCE(option82_circuit_id, ''),
			COALESCE(option82_remote_id, ''),
			COALESCE(option82_vlan, ''),
			COALESCE(relay_ip::text, ''),
			COALESCE(relay_switch_id::text, ''),
			COALESCE(relay_switch_name, ''),
			message_type,
			COALESCE(source_ip::text, ''),
			observed_at,
			created_at
		FROM dhcp_events
		WHERE mac_address = $1
		  AND message_type = $2
		  AND transaction_id = $3
		  AND created_at >= $4
		ORDER BY created_at DESC
		LIMIT 1
	`

	var event domain.Event
	if err := r.pool.QueryRow(ctx, query, macAddress, messageType, transactionID, since).Scan(
		&event.ID,
		&event.MACAddress,
		&event.TransactionID,
		&event.ClientIP,
		&event.YourIP,
		&event.RequestedIP,
		&event.Hostname,
		&event.VendorClass,
		&event.Option82Raw,
		&event.Option82CircuitID,
		&event.Option82RemoteID,
		&event.Option82VLAN,
		&event.RelayIP,
		&event.RelaySwitchID,
		&event.RelaySwitchName,
		&event.MessageType,
		&event.SourceIP,
		&event.ObservedAt,
		&event.CreatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &event, nil
}

func (r *PostgresRepository) FindRecent(ctx context.Context, macAddress, messageType string, since time.Time) (*domain.Event, error) {
	query := `
		SELECT
			id,
			mac_address,
			COALESCE(transaction_id, ''),
			COALESCE(client_ip::text, ''),
			COALESCE(your_ip::text, ''),
			COALESCE(requested_ip::text, ''),
			COALESCE(hostname, ''),
			COALESCE(vendor_class, ''),
			COALESCE(option82_raw, ''),
			COALESCE(option82_circuit_id, ''),
			COALESCE(option82_remote_id, ''),
			COALESCE(option82_vlan, ''),
			COALESCE(relay_ip::text, ''),
			COALESCE(relay_switch_id::text, ''),
			COALESCE(relay_switch_name, ''),
			message_type,
			COALESCE(source_ip::text, ''),
			observed_at,
			created_at
		FROM dhcp_events
		WHERE mac_address = $1
		  AND message_type = $2
		  AND created_at >= $3
		ORDER BY created_at DESC
		LIMIT 1
	`

	var event domain.Event
	if err := r.pool.QueryRow(ctx, query, macAddress, messageType, since).Scan(
		&event.ID,
		&event.MACAddress,
		&event.TransactionID,
		&event.ClientIP,
		&event.YourIP,
		&event.RequestedIP,
		&event.Hostname,
		&event.VendorClass,
		&event.Option82Raw,
		&event.Option82CircuitID,
		&event.Option82RemoteID,
		&event.Option82VLAN,
		&event.RelayIP,
		&event.RelaySwitchID,
		&event.RelaySwitchName,
		&event.MessageType,
		&event.SourceIP,
		&event.ObservedAt,
		&event.CreatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &event, nil
}

func (r *PostgresRepository) Insert(ctx context.Context, event domain.Event) (domain.Event, error) {
	query := `
		INSERT INTO dhcp_events (
			id,
			mac_address,
			transaction_id,
			source_ip,
			client_ip,
			your_ip,
			requested_ip,
			message_type,
			hostname,
			vendor_class,
			option82_raw,
			option82_circuit_id,
			option82_remote_id,
			option82_vlan,
			relay_ip,
			relay_switch_id,
			relay_switch_name,
			observed_at,
			created_at
		)
		VALUES ($1, NULLIF($2, ''), $3, NULLIF($4, '')::inet, NULLIF($5, '')::inet, NULLIF($6, '')::inet, NULLIF($7, '')::inet, $8, $9, $10, $11, $12, $13, $14, NULLIF($15, '')::inet, NULLIF($16, '')::uuid, $17, $18, $19)
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		event.ID,
		event.MACAddress,
		event.TransactionID,
		event.SourceIP,
		event.ClientIP,
		event.YourIP,
		event.RequestedIP,
		event.MessageType,
		event.Hostname,
		event.VendorClass,
		event.Option82Raw,
		event.Option82CircuitID,
		event.Option82RemoteID,
		event.Option82VLAN,
		event.RelayIP,
		event.RelaySwitchID,
		event.RelaySwitchName,
		event.ObservedAt,
		event.CreatedAt,
	)
	if err != nil {
		return domain.Event{}, err
	}

	return event, nil
}

func (r *PostgresRepository) UpdateByID(ctx context.Context, id string, event domain.Event) (domain.Event, error) {
	query := `
		UPDATE dhcp_events
		SET transaction_id = $2,
		    source_ip = NULLIF($3, '')::inet,
		    client_ip = NULLIF($4, '')::inet,
		    your_ip = NULLIF($5, '')::inet,
		    requested_ip = NULLIF($6, '')::inet,
		    hostname = $7,
		    vendor_class = $8,
		    option82_raw = $9,
		    option82_circuit_id = $10,
		    option82_remote_id = $11,
		    option82_vlan = $12,
		    relay_ip = NULLIF($13, '')::inet,
		    relay_switch_id = NULLIF($14, '')::uuid,
		    relay_switch_name = $15,
		    observed_at = $16,
		    created_at = $17
		WHERE id = $1
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		id,
		event.TransactionID,
		event.SourceIP,
		event.ClientIP,
		event.YourIP,
		event.RequestedIP,
		event.Hostname,
		event.VendorClass,
		event.Option82Raw,
		event.Option82CircuitID,
		event.Option82RemoteID,
		event.Option82VLAN,
		event.RelayIP,
		event.RelaySwitchID,
		event.RelaySwitchName,
		event.ObservedAt,
		event.CreatedAt,
	)
	if err != nil {
		return domain.Event{}, err
	}

	event.ID = id
	return event, nil
}
