package switchasset

import (
	"context"
	"time"
)

type Repository interface {
	Insert(ctx context.Context, asset Switch) (Switch, error)
	List(ctx context.Context) ([]Switch, error)
	ListEnabledSNMP(ctx context.Context) ([]Switch, error)
	FindByID(ctx context.Context, id string) (*Switch, error)
	FindByName(ctx context.Context, name string) (*Switch, error)
	FindByManagementIP(ctx context.Context, managementIP string) (*Switch, error)
	FindByBaseMAC(ctx context.Context, macAddress string) (*Switch, error)
	FindByNeighborName(ctx context.Context, name string) (*Switch, error)
	UpdateIdentity(ctx context.Context, id, systemName, baseMAC string, aliases []string) (Switch, error)
	UpdateRoutingSwitch(ctx context.Context, id, routingSwitchID string) (Switch, error)
	UpdateRadiusSecret(ctx context.Context, id, radiusSecret string) (Switch, error)
	UpdateSSHConfig(ctx context.Context, id, username, password string, port int) (Switch, error)
	UpdatePollStatus(ctx context.Context, id string, polledAt time.Time, lastError string) (Switch, error)
}
