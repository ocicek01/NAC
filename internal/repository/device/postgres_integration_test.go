package device

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"nac/internal/config"
	"nac/internal/database"
	domain "nac/internal/domain/device"
)

func TestUpsertReturnsCompleteDeviceProjection(t *testing.T) {
	if os.Getenv("NAC_RUN_DB_TESTS") != "1" {
		t.Skip("requires a migrated PostgreSQL database and NAC_RUN_DB_TESTS=1")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	random := uuid.New()
	now := time.Now().UTC()
	input := domain.Device{
		ID: uuid.NewString(), MACAddress: fmt.Sprintf("02:%02X:%02X:%02X:%02X:%02X", random[0], random[1], random[2], random[3], random[4]),
		DeviceType: "unknown", Label: "upsert-projection-regression", Status: "unknown",
		AuthenticationMethod: "mab", AuthenticationStatus: "pending",
		FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, "DELETE FROM devices WHERE id=$1 AND mac_address=$2", input.ID, input.MACAddress); err != nil {
			t.Errorf("cleanup temporary device: %v", err)
		}
	}()
	repo := NewPostgresRepository(pool)
	for _, label := range []string{input.Label, "upsert-projection-updated"} {
		input.Label = label
		result, err := repo.Upsert(ctx, input)
		if err != nil {
			t.Fatalf("upsert complete projection: %v", err)
		}
		if result.ID != input.ID || result.MACAddress != input.MACAddress || result.Label != label || result.AuthenticationMethod != "mab" {
			t.Fatalf("incorrect returned device fields: id=%s mac=%s label=%s method=%s", result.ID, result.MACAddress, result.Label, result.AuthenticationMethod)
		}
		if result.CurrentManagementIP != "" || result.CurrentReasonCodes == nil {
			t.Fatal("expected empty management IP and initialized derived reason codes")
		}
		stored, err := repo.FindByID(ctx, input.ID)
		if err != nil || stored == nil || stored.Label != result.Label || stored.AuthenticationMethod != result.AuthenticationMethod {
			t.Fatalf("upsert/find projections differ: %v", err)
		}
	}
}
