package sophosauth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/sophosauth"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

type postgresEffects struct {
	tx    pgx.Tx
	claim domain.Claim
}

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
			correlated_mac, correlation_status, processing_status, metadata, event_at, received_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::inet,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (fingerprint) DO NOTHING
		RETURNING id, fingerprint, event_type, message_id, username, user_group,
			client_used, auth_mechanism, auth_status, COALESCE(host(source_ip),''),
			source_mac, correlated_mac, correlation_status, processing_status,
			attempt_count, last_error, next_attempt_at, processed_at, metadata, event_at, received_at
	`, event.ID, event.Fingerprint, event.EventType, event.MessageID, event.Username,
		event.UserGroup, event.ClientUsed, event.AuthMechanism, event.AuthStatus,
		event.SourceIP, event.SourceMAC, event.CorrelatedMAC, event.CorrelationStatus,
		processingStatus(event), metadata, event.EventAt, event.ReceivedAt))
	if err == pgx.ErrNoRows {
		stored, findErr := r.findEventByFingerprint(ctx, event.Fingerprint)
		return stored, false, findErr
	}
	return stored, err == nil, err
}

func processingStatus(event domain.Event) string {
	if event.EventType == "ignored" {
		return "ignored"
	}
	return "pending"
}

func (r *PostgresRepository) ClaimEvent(ctx context.Context, fingerprint string, now time.Time, lease time.Duration, maxAttempts int) (domain.Claim, bool, error) {
	return r.claimOne(ctx, fingerprint, now, lease, maxAttempts)
}

func (r *PostgresRepository) ClaimDueEvents(ctx context.Context, now time.Time, lease time.Duration, maxAttempts, limit int) ([]domain.Claim, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	claims := make([]domain.Claim, 0, limit)
	for len(claims) < limit {
		claim, ok, err := r.claimOne(ctx, "", now, lease, maxAttempts)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		claims = append(claims, claim)
	}
	return claims, nil
}

func (r *PostgresRepository) claimOne(ctx context.Context, fingerprint string, now time.Time, lease time.Duration, maxAttempts int) (domain.Claim, bool, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	token := uuid.NewString()
	event, err := scanEvent(r.pool.QueryRow(ctx, `
		WITH candidate AS (
			SELECT e.id FROM sophos_auth_events e
			WHERE ($1='' OR e.fingerprint=$1)
			  AND e.attempt_count < $5
			  AND (
				e.processing_status='pending'
				OR (e.processing_status='retry' AND COALESCE(e.next_attempt_at, $2) <= $2)
				OR (e.processing_status='processing' AND e.lease_until < $2)
			  )
			  AND NOT EXISTS (
				SELECT 1 FROM sophos_auth_events predecessor
				WHERE lower(predecessor.username)=lower(e.username)
				  AND predecessor.source_ip=e.source_ip
				  AND predecessor.id<>e.id
				  AND predecessor.processing_status IN ('pending','retry','processing')
				  AND (
					predecessor.event_at < e.event_at
					OR (predecessor.event_at=e.event_at
						AND CASE WHEN predecessor.event_type='logout' THEN 2 ELSE 1 END
							< CASE WHEN e.event_type='logout' THEN 2 ELSE 1 END)
					OR (predecessor.event_at=e.event_at
						AND CASE WHEN predecessor.event_type='logout' THEN 2 ELSE 1 END
							= CASE WHEN e.event_type='logout' THEN 2 ELSE 1 END
						AND predecessor.received_at < e.received_at)
					OR (predecessor.event_at=e.event_at
						AND CASE WHEN predecessor.event_type='logout' THEN 2 ELSE 1 END
							= CASE WHEN e.event_type='logout' THEN 2 ELSE 1 END
						AND predecessor.received_at=e.received_at AND predecessor.id < e.id)
				  )
			  )
			ORDER BY e.event_at, CASE WHEN e.event_type='logout' THEN 2 ELSE 1 END, e.received_at, e.id
			FOR UPDATE SKIP LOCKED LIMIT 1
		)
		UPDATE sophos_auth_events e
		SET processing_status='processing', attempt_count=attempt_count+1,
			lease_token=$3, lease_until=$2+$4::interval, last_error='', updated_at=$2
		FROM candidate WHERE e.id=candidate.id
		RETURNING e.id, e.fingerprint, e.event_type, e.message_id, e.username, e.user_group,
			e.client_used, e.auth_mechanism, e.auth_status, COALESCE(host(e.source_ip),''),
			e.source_mac, e.correlated_mac, e.correlation_status, e.processing_status,
			e.attempt_count, e.last_error, e.next_attempt_at, e.processed_at,
			e.metadata, e.event_at, e.received_at
	`, strings.TrimSpace(fingerprint), now, token, intervalLiteral(lease), maxAttempts))
	if err == pgx.ErrNoRows {
		return domain.Claim{}, false, nil
	}
	if err != nil {
		return domain.Claim{}, false, err
	}
	return domain.Claim{Event: event, Token: token}, true, nil
}

func (e *postgresEffects) ReserveCausalOrder(ctx context.Context, event domain.Event) (bool, error) {
	eventRank := 1
	if event.EventType == "logout" {
		eventRank = 2
	}
	var accepted bool
	err := e.tx.QueryRow(ctx, `
		INSERT INTO sophos_auth_identity_watermarks (
			username_normalized, source_ip, last_event_at, last_event_rank,
			last_received_at, last_event_id, last_event_type, updated_at
		) VALUES (lower($1),NULLIF($2,'')::inet,$3,$4,$5,$6,$7,NOW())
		ON CONFLICT (username_normalized, source_ip) DO UPDATE SET
			last_event_at=EXCLUDED.last_event_at,
			last_event_rank=EXCLUDED.last_event_rank,
			last_received_at=EXCLUDED.last_received_at,
			last_event_id=EXCLUDED.last_event_id,
			last_event_type=EXCLUDED.last_event_type,
			updated_at=NOW()
		WHERE (EXCLUDED.last_event_at, EXCLUDED.last_event_rank, EXCLUDED.last_received_at, EXCLUDED.last_event_id)
			> (sophos_auth_identity_watermarks.last_event_at,
			   sophos_auth_identity_watermarks.last_event_rank,
			   sophos_auth_identity_watermarks.last_received_at,
			   sophos_auth_identity_watermarks.last_event_id)
		RETURNING true
	`, strings.TrimSpace(event.Username), strings.TrimSpace(event.SourceIP), event.EventAt,
		eventRank, event.ReceivedAt, event.ID, event.EventType).Scan(&accepted)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return accepted, err
}

func intervalLiteral(value time.Duration) string {
	if value <= 0 {
		value = time.Minute
	}
	return fmt.Sprintf("%f seconds", value.Seconds())
}

func (r *PostgresRepository) BeginEffects(ctx context.Context, claim domain.Claim) (domain.Effects, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	var valid bool
	err = tx.QueryRow(ctx, `
		SELECT true FROM sophos_auth_events
		WHERE id=$1 AND processing_status='processing' AND lease_token=$2 AND lease_until >= NOW()
		FOR UPDATE
	`, claim.Event.ID, claim.Token).Scan(&valid)
	if err != nil || !valid {
		_ = tx.Rollback(ctx)
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("Sophos auth event claim is no longer valid")
		}
		return nil, err
	}
	return &postgresEffects{tx: tx, claim: claim}, nil
}

func (r *PostgresRepository) MarkFailed(ctx context.Context, claim domain.Claim, message string, nextAttempt time.Time, maxAttempts int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE sophos_auth_events SET
			processing_status=CASE WHEN attempt_count >= $5 THEN 'dead_letter' ELSE 'retry' END,
			last_error=$3,
			next_attempt_at=CASE WHEN attempt_count >= $5 THEN NULL ELSE $4 END,
			lease_token='', lease_until=NULL, updated_at=NOW()
		WHERE id=$1 AND processing_status='processing' AND lease_token=$2
	`, claim.Event.ID, claim.Token, truncateError(message), nextAttempt, maxAttempts)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("Sophos auth event claim was lost before failure state persisted")
	}
	return nil
}

func truncateError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 2000 {
		return value[:2000]
	}
	return value
}

func (e *postgresEffects) UpdateEventCorrelation(ctx context.Context, id, status, macAddress string, metadata map[string]any) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = e.tx.Exec(ctx, `
		UPDATE sophos_auth_events
		SET correlation_status=$2, correlated_mac=$3, metadata=metadata || $4::jsonb
		WHERE id=$1
	`, id, status, strings.ToUpper(strings.TrimSpace(macAddress)), payload)
	return err
}

func (e *postgresEffects) FindFreshMACsByIP(ctx context.Context, ipAddress string, since time.Time) ([]string, error) {
	rows, err := e.tx.Query(ctx, `
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

func (e *postgresEffects) OpenSession(ctx context.Context, event domain.Event) (domain.Session, error) {
	activeKey := strings.ToLower(strings.TrimSpace(event.Username)) + "|" + strings.TrimSpace(event.SourceIP)
	return scanSession(e.tx.QueryRow(ctx, `
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

func (e *postgresEffects) CloseSessions(ctx context.Context, username, ipAddress string, endedAt time.Time) ([]domain.Session, error) {
	rows, err := e.tx.Query(ctx, `
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

func (e *postgresEffects) SetDeviceIdentity(ctx context.Context, macAddress, username, ipAddress string, seenAt time.Time) error {
	_, err := e.tx.Exec(ctx, `
		UPDATE devices SET sophos_username=$2, sophos_last_ip=NULLIF($3,'')::inet,
			sophos_last_seen_at=$4, authentication_status='captive-authenticated',
			authentication_method='sophos_captive_portal', updated_at=NOW()
		WHERE regexp_replace(upper(mac_address),'[^0-9A-F]','','g')=regexp_replace(upper($1),'[^0-9A-F]','','g')
		  AND (sophos_last_seen_at IS NULL OR sophos_last_seen_at <= $4)
	`, macAddress, username, ipAddress, seenAt)
	return err
}

func (e *postgresEffects) ClearDeviceIdentityIfMatch(ctx context.Context, macAddress, username, ipAddress string, logoutAt time.Time) error {
	_, err := e.tx.Exec(ctx, `
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

func (e *postgresEffects) RecordAudit(ctx context.Context, action, status, eventID, macAddress string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = e.tx.Exec(ctx, `
		INSERT INTO audit_logs (
			id, actor, action, status, target_type, target_id, mac_address, payload, created_at
		) VALUES ($1,'system',$2,$3,'sophos_captive_session',$4,$5,$6::jsonb,NOW())
	`, uuid.NewString(), action, status, eventID, macAddress, encoded)
	return err
}

func (e *postgresEffects) Complete(ctx context.Context) error {
	tag, err := e.tx.Exec(ctx, `
		UPDATE sophos_auth_events SET processing_status='completed', processed_at=NOW(),
			next_attempt_at=NULL, lease_token='', lease_until=NULL, last_error='', updated_at=NOW()
		WHERE id=$1 AND processing_status='processing' AND lease_token=$2
	`, e.claim.Event.ID, e.claim.Token)
	if err != nil {
		_ = e.tx.Rollback(ctx)
		return err
	}
	if tag.RowsAffected() != 1 {
		_ = e.tx.Rollback(ctx)
		return fmt.Errorf("Sophos auth event claim was lost before completion")
	}
	return e.tx.Commit(ctx)
}

func (e *postgresEffects) Rollback(ctx context.Context) error {
	err := e.tx.Rollback(ctx)
	if err == pgx.ErrTxClosed {
		return nil
	}
	return err
}

func (r *PostgresRepository) ListEvents(ctx context.Context, limit, offset int) ([]domain.Event, error) {
	limit, offset = bounds(limit, offset)
	rows, err := r.pool.Query(ctx, `
		SELECT id, fingerprint, event_type, message_id, username, user_group,
			client_used, auth_mechanism, auth_status, COALESCE(host(source_ip),''),
			source_mac, correlated_mac, correlation_status, processing_status,
			attempt_count, last_error, next_attempt_at, processed_at, metadata, event_at, received_at
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
			max(event_at), max(event_at) FILTER (WHERE correlation_status='correlated'),
			count(*) FILTER (WHERE processing_status IN ('pending','retry','processing')),
			count(*) FILTER (WHERE processing_status='dead_letter')
		FROM sophos_auth_events
	`).Scan(&health.Uncorrelated, &health.Ambiguous, &health.LastEventAt, &health.LastCorrelatedAt,
		&health.RetryPending, &health.DeadLetter)
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
			source_mac, correlated_mac, correlation_status, processing_status,
			attempt_count, last_error, next_attempt_at, processed_at, metadata, event_at, received_at
		FROM sophos_auth_events WHERE fingerprint=$1
	`, fingerprint))
}

type scanner interface{ Scan(...any) error }

func scanEvent(row scanner) (domain.Event, error) {
	var item domain.Event
	err := row.Scan(&item.ID, &item.Fingerprint, &item.EventType, &item.MessageID,
		&item.Username, &item.UserGroup, &item.ClientUsed, &item.AuthMechanism,
		&item.AuthStatus, &item.SourceIP, &item.SourceMAC, &item.CorrelatedMAC,
		&item.CorrelationStatus, &item.ProcessingStatus, &item.AttemptCount,
		&item.LastError, &item.NextAttemptAt, &item.ProcessedAt, &item.Metadata,
		&item.EventAt, &item.ReceivedAt)
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
