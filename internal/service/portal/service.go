package portal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	device "nac/internal/domain/device"
	guestdomain "nac/internal/domain/guestidentity"
	"nac/internal/normalize"
	identitysource "nac/internal/service/identitysource"
)

type DeviceRegistry interface {
	ListByMAC(ctx context.Context, macAddress string) ([]device.Device, error)
	UpdateStatus(ctx context.Context, macAddress, status, approvedBy string, expiresAt time.Time, targetVLAN int) (device.Device, error)
	AddIdentitySnapshot(ctx context.Context, snapshot device.IdentitySnapshot) (device.IdentitySnapshot, error)
}

type GuestResolver interface {
	FindActiveByIdentifier(ctx context.Context, identifier string) (*guestdomain.Identity, error)
}

type Service struct {
	devices   DeviceRegistry
	ldap      identitysource.Resolver
	staff     identitysource.Resolver
	student   identitysource.Resolver
	guests    GuestResolver
	guestVLAN int
}

func NewService(devices DeviceRegistry, ldap, staff, student identitysource.Resolver, guests GuestResolver, guestVLAN int) *Service {
	return &Service{
		devices:   devices,
		ldap:      ldap,
		staff:     staff,
		student:   student,
		guests:    guests,
		guestVLAN: guestVLAN,
	}
}

type RegistrationInput struct {
	MACAddress string
	Identifier string
	Password   string
	ApprovedBy string
}

type RegistrationResult struct {
	Device        device.Device            `json:"device"`
	MatchedSource string                   `json:"matched_source"`
	IdentityType  string                   `json:"identity_type"`
	Snapshot      *device.IdentitySnapshot `json:"snapshot,omitempty"`
}

func (s *Service) Register(ctx context.Context, input RegistrationInput) (RegistrationResult, error) {
	macAddress := normalize.MACAddress(input.MACAddress)
	identifier := strings.TrimSpace(input.Identifier)
	if macAddress == "" {
		return RegistrationResult{}, fmt.Errorf("mac_address is required")
	}
	if identifier == "" {
		return RegistrationResult{}, fmt.Errorf("identifier is required")
	}

	devices, err := s.devices.ListByMAC(ctx, macAddress)
	if err != nil {
		return RegistrationResult{}, err
	}
	if len(devices) == 0 {
		return RegistrationResult{}, fmt.Errorf("device not found")
	}
	current := devices[0]

	if result, err := s.tryIdentitySources(ctx, identifier, input.Password); err != nil {
		return RegistrationResult{}, err
	} else if result != nil {
		targetVLAN := result.TargetVLAN
		if targetVLAN <= 0 {
			targetVLAN = s.guestVLAN
		}

		snapshot, err := s.devices.AddIdentitySnapshot(ctx, device.IdentitySnapshot{
			ID:             uuid.NewString(),
			DeviceID:       current.ID,
			IdentityType:   result.IdentityType,
			IdentitySource: result.Source,
			ExternalID:     result.ExternalID,
			Username:       result.Username,
			FullName:       result.FullName,
			Attributes:     result.Attributes,
			VerifiedAt:     time.Now().UTC(),
			ExpiresAt:      result.ExpiresAt,
		})
		if err != nil {
			return RegistrationResult{}, err
		}

		updated, err := s.devices.UpdateStatus(ctx, macAddress, "allowed", input.ApprovedBy, result.ExpiresAt, targetVLAN)
		if err != nil {
			return RegistrationResult{}, err
		}

		return RegistrationResult{
			Device:        updated,
			MatchedSource: result.Source,
			IdentityType:  result.IdentityType,
			Snapshot:      &snapshot,
		}, nil
	}

	guest, err := s.guests.FindActiveByIdentifier(ctx, identifier)
	if err != nil {
		return RegistrationResult{}, err
	}
	if guest == nil {
		updated, updateErr := s.devices.UpdateStatus(ctx, macAddress, "blocked", input.ApprovedBy, time.Time{}, 0)
		if updateErr != nil {
			return RegistrationResult{}, updateErr
		}
		return RegistrationResult{
			Device:        updated,
			MatchedSource: "",
			IdentityType:  "",
		}, nil
	}

	expiresAt := guest.ExpiresAt
	targetVLAN := guest.TargetVLAN
	if targetVLAN <= 0 {
		targetVLAN = s.guestVLAN
	}

	snapshot, err := s.devices.AddIdentitySnapshot(ctx, device.IdentitySnapshot{
		ID:             uuid.NewString(),
		DeviceID:       current.ID,
		IdentityType:   "misafir",
		IdentitySource: "guest_registry",
		ExternalID:     guest.ExternalID,
		Username:       guest.Username,
		FullName:       guest.FullName,
		Attributes: map[string]any{
			"email": guest.Email,
			"phone": guest.Phone,
		},
		VerifiedAt: time.Now().UTC(),
		ExpiresAt:  expiresAt,
	})
	if err != nil {
		return RegistrationResult{}, err
	}

	updated, err := s.devices.UpdateStatus(ctx, macAddress, "allowed", input.ApprovedBy, expiresAt, targetVLAN)
	if err != nil {
		return RegistrationResult{}, err
	}

	return RegistrationResult{
		Device:        updated,
		MatchedSource: "guest_registry",
		IdentityType:  "misafir",
		Snapshot:      &snapshot,
	}, nil
}

func (s *Service) tryIdentitySources(ctx context.Context, identifier, password string) (*identitysource.Result, error) {
	for _, resolver := range []identitysource.Resolver{s.ldap, s.staff, s.student} {
		if resolver == nil {
			continue
		}
		result, err := resolver.Resolve(ctx, identifier, password)
		if err != nil {
			return nil, err
		}
		if result != nil && result.Matched {
			if result.Attributes == nil {
				result.Attributes = map[string]any{}
			}
			return result, nil
		}
	}
	return nil, nil
}
