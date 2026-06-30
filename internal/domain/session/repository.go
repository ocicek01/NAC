package session

import "context"

type Repository interface {
	Upsert(ctx context.Context, session Session) (Session, error)
	ListRecent(ctx context.Context, limit int) ([]Session, error)
	ListRecentByMAC(ctx context.Context, macAddress string, limit int) ([]Session, error)
	FindByAcctSession(ctx context.Context, macAddress, switchID, acctSessionID string) (*Session, error)
	FindLatestActiveByMACSwitch(ctx context.Context, macAddress, switchID string) (*Session, error)
	PromoteToAccountingKey(ctx context.Context, oldKey, newKey, acctSessionID string) error
}
