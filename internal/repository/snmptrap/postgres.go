package snmptrap

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	domain "nac/internal/domain/snmptrap"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Insert(ctx context.Context, event domain.Event) (domain.Event, error) {
	varBinds, err := json.Marshal(event.VarBinds)
	if err != nil {
		return domain.Event{}, err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO snmp_trap_events (
			id, source_ip, source_port, switch_id, switch_name, snmp_version, community,
			trap_oid, enterprise_oid, generic_trap, specific_trap, uptime_ticks,
			varbinds, received_at, created_at
		)
		VALUES (
			$1, NULLIF($2, '')::inet, $3, NULLIF($4, '')::uuid, $5, $6, $7,
			$8, $9, $10, $11, $12, $13::jsonb, $14, $15
		)
	`,
		event.ID,
		event.SourceIP,
		event.SourcePort,
		event.SwitchID,
		event.SwitchName,
		event.SNMPVersion,
		event.Community,
		event.TrapOID,
		event.EnterpriseOID,
		event.GenericTrap,
		event.SpecificTrap,
		event.UptimeTicks,
		string(varBinds),
		event.ReceivedAt,
		event.CreatedAt,
	)
	if err != nil {
		return domain.Event{}, err
	}

	return event, nil
}
