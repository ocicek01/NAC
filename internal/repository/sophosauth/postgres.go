package sophosauth

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/sophosauth"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) InsertEvent(ctx context.Context, event domain.Event) (domain.Event, bool, error) {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return domain.Event{}, false, err
	}
	stored, err := scanEvent(r.pool.QueryRow(ctx, `
		INSERT INTO sophos_auth_events (
			id, fingerprint, event_type, message_id, username, user_group,
			client_used, auth_mechanism, auth_status, source_ip, source_mac,
			correlated_mac, correlation_status, metadata, event_at, received_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::inet,$11,$12,$13,$14,$15,$16)
		ON CONFLICT (fingerprint) DO NOTHING
		RETURNING id, fingerprint, event_type, message_id, username, user_group,
			client_used, auth_mechanism, auth_status, COALESCE(host(source_ip),''),
			source_mac, correlated_mac, correlation_status, metadata, event_at, received_at
	`, event.ID, event.Fingerprint, event.EventType, event.MessageID, event.Username,
		event.UserGroup, event.ClientUsed, event.AuthMechanism, event.AuthStatus,
		event.SourceIP, event.SourceMAC, event.CorrelatedMAC, event.CorrelationStatus,
		metadata, event.EventAt, event.ReceivedAt))
	if err == pgx.ErrNoRows {
		stored, findErr := r.findEventByFingerprint(ctx, event.Fingerprint)
		return stored, false, findErr
	}
	return stored, err == nil, err
}

func (r *PostgresRepository) UpdateEventCorrelation(ctx context.Context, id, status, macAddress string, metadata map[string]any) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE sophos_auth_events
		SET correlation_status=$2, correlated_mac=$3, metadata=metadata || $4::jsonb
		WHERE id=$1
	`, id, status, strings.ToUpper(strings.TrimSpace(macAddress)), payload)
	return err
}

func (r *PostgresRepository) FindFreshMACsByIP(ctx context.Context, ipAddress string, since time.Time) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT upper(mac_address)
		FROM mac_ip_bindings
		WHERE ip_address=NULLIF($1,'')::inet AND last_seen_at >= $2
		ORDER BY upper(mac_address)
	`, strings.TrimSpace(ipAddress), since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0, 2)
	for rows.Next() {
		var mac string
		if err := rows.Scan(&mac); err != nil {
			return nil, err
		}
		items = append(items, mac)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) OpenSession(ctx context.Context, event domain.Event) (domain.Session, error) {
	activeKey := strings.ToLower(strings.TrimSpace(event.Username)) + "|" + strings.TrimSpace(event.SourceIP)
	return scanSession(r.pool.QueryRow(ctx, `
		INSERT INTO sophos_user_sessions (
			id, active_key, username, user_group, ip_address, mac_address,
			client_used, auth_mechanism, status, correlation_status,
			started_at, last_seen_at, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5::inet,$6,$7,$8,'active',$9,$10,$10,$11,$11)
		ON CONFLICT (active_key) DO UPDATE SET
			user_group=CASE WHEN EXCLUDED.started_at >= sophos_user_sessions.started_at THEN EXCLUDED.user_group ELSE sophos_user_sessions.user_group END,
			mac_address=CASE WHEN EXCLUDED.started_at >= sophos_user_sessions.started_at THEN EXCLUDED.mac_address ELSE sophos_user_sessions.mac_address END,
			client_used=CASE WHEN EXCLUDED.started_at >= sophos_user_sessions.started_at THEN EXCLUDED.client_used ELSE sophos_user_sessions.client_used END,
			auth_mechanism=CASE WHEN EXCLUDED.started_at >= sophos_user_sessions.started_at THEN EXCLUDED.auth_mechanism ELSE sophos_user_sessions.auth_mechanism END,
			correlation_status=CASE WHEN EXCLUDED.started_at >= sophos_user_sessions.started_at THEN EXCLUDED.correlation_status ELSE sophos_user_sessions.correlation_status END,
			started_at=GREATEST(sophos_user_sessions.started_at, EXCLUDED.started_at),
			last_seen_at=GREATEST(sophos_user_sessions.last_seen_at, EXCLUDED.last_seen_at),
			updated_at=GREATEST(sophos_user_sessions.updated_at, EXCLUDED.updated_at)
		RETURNING id, username, user_group, host(ip_address), mac_address,
			client_used, auth_mechanism, status, correlation_status,
			started_at, last_seen_at, ended_at, created_at, updated_at
	`, uuid.NewString(), activeKey, event.Username, event.UserGroup, event.SourceIP,
		event.CorrelatedMAC, event.ClientUsed, event.AuthMechanism,
		event.CorrelationStatus, event.EventAt, event.ReceivedAt))
}

func (r *PostgresRepository) CloseSessions(ctx context.Context, username, ipAddress string, endedAt time.Time) ([]domain.Session, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE sophos_user_sessions
		SET status='closed', active_key=NULL, ended_at=$3,
			last_seen_at=GREATEST(last_seen_at,$3), updated_at=NOW()
		WHERE status='active' AND lower(username)=lower($1) AND ip_address=NULLIF($2,'')::inet
		  AND started_at <= $3
		RETURNING id, username, user_group, host(ip_address), mac_address,
			client_used, auth_mechanism, status, correlation_status,
			started_at, last_seen_at, ended_at, created_at, updated_at
	`, strings.TrimSpace(username), strings.TrimSpace(ipAddress), endedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Session, 0, 1)
	for rows.Next() {
		item, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) SetDeviceIdentity(ctx context.Context, macAddress, username, ipAddress string, seenAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE devices SET sophos_username=$2, sophos_last_ip=NULLIF($3,'')::inet,
			sophos_last_seen_at=$4, authentication_status='captive-authenticated',
			authentication_method='sophos_captive_portal', updated_at=NOW()
		WHERE regexp_replace(upper(mac_address),'[^0-9A-F]','','g')=regexp_replace(upper($1),'[^0-9A-F]','','g')
		  AND (sophos_last_seen_at IS NULL OR sophos_last_seen_at <= $4)
	`, macAddress, username, ipAddress, seenAt)
	return err
}

func (r *PostgresRepository) ClearDeviceIdentityIfMatch(ctx context.Context, macAddress, username, ipAddress string, logoutAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE devices SET sophos_username='', sophos_last_ip=NULL,
			sophos_last_seen_at=$4,
			authentication_status=CASE WHEN authentication_status='captive-authenticated' THEN 'not_attempted' ELSE authentication_status END,
			authentication_method=CASE WHEN authentication_method='sophos_captive_portal' THEN '' ELSE authentication_method END,
			updated_at=NOW()
		WHERE regexp_replace(upper(mac_address),'[^0-9A-F]','','g')=regexp_replace(upper($1),'[^0-9A-F]','','g')
		  AND lower(sophos_username)=lower($2) AND sophos_last_ip=NULLIF($3,'')::inet
		  AND sophos_last_seen_at <= $4
	`, macAddress, username, ipAddress, logoutAt)
	return err
}

func (r *PostgresRepository) ListEvents(ctx context.Context, limit, offset int) ([]domain.Event, error) {
	limit, offset = bounds(limit, offset)
	rows, err := r.pool.Query(ctx, `
		SELECT id, fingerprint, event_type, message_id, username, user_group,
			client_used, auth_mechanism, auth_status, COALESCE(host(source_ip),''),
			source_mac, correlated_mac, correlation_status, metadata, event_at, received_at
		FROM sophos_auth_events ORDER BY received_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Event, 0, limit)
	for rows.Next() {
		item, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) ListSessions(ctx context.Context, limit, offset int) ([]domain.Session, error) {
	limit, offset = bounds(limit, offset)
	rows, err := r.pool.Query(ctx, `
		SELECT id, username, user_group, host(ip_address), mac_address,
			client_used, auth_mechanism, status, correlation_status,
			started_at, last_seen_at, ended_at, created_at, updated_at
		FROM sophos_user_sessions ORDER BY last_seen_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Session, 0, limit)
	for rows.Next() {
		item, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) Health(ctx context.Context, enabled bool, bindAddress string, allowedSources int) (domain.Health, error) {
	health := domain.Health{Enabled: enabled, BindAddress: bindAddress, AllowedSources: allowedSources}
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE correlation_status='uncorrelated'),
			count(*) FILTER (WHERE correlation_status='ambiguous'),
			max(event_at), max(event_at) FILTER (WHERE correlation_status='correlated')
		FROM sophos_auth_events
	`).Scan(&health.Uncorrelated, &health.Ambiguous, &health.LastEventAt, &health.LastCorrelatedAt)
	if err != nil {
		// Active session count needs a separate aggregate to avoid a cross product.
		return domain.Health{}, err
	}
	err = r.pool.QueryRow(ctx, `SELECT count(*) FROM sophos_user_sessions WHERE status='active'`).Scan(&health.ActiveSessions)
	return health, err
}

func (r *PostgresRepository) findEventByFingerprint(ctx context.Context, fingerprint string) (domain.Event, error) {
	return scanEvent(r.pool.QueryRow(ctx, `
		SELECT id, fingerprint, event_type, message_id, username, user_group,
			client_used, auth_mechanism, auth_status, COALESCE(host(source_ip),''),
			source_mac, correlated_mac, correlation_status, metadata, event_at, received_at
		FROM sophos_auth_events WHERE fingerprint=$1
	`, fingerprint))
}

type scanner interface{ Scan(...any) error }

func scanEvent(row scanner) (domain.Event, error) {
	var item domain.Event
	err := row.Scan(&item.ID, &item.Fingerprint, &item.EventType, &item.MessageID,
		&item.Username, &item.UserGroup, &item.ClientUsed, &item.AuthMechanism,
		&item.AuthStatus, &item.SourceIP, &item.SourceMAC, &item.CorrelatedMAC,
		&item.CorrelationStatus, &item.Metadata, &item.EventAt, &item.ReceivedAt)
	return item, err
}

func scanSession(row scanner) (domain.Session, error) {
	var item domain.Session
	err := row.Scan(&item.ID, &item.Username, &item.UserGroup, &item.IPAddress,
		&item.MACAddress, &item.ClientUsed, &item.AuthMechanism, &item.Status,
		&item.CorrelationStatus, &item.StartedAt, &item.LastSeenAt, &item.EndedAt,
		&item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func bounds(limit, offset int) (int, int) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
