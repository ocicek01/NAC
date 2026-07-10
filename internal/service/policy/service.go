package policy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "nac/internal/domain/policy"
	auditlogservice "nac/internal/service/auditlog"
)

type Repository interface {
	Insert(ctx context.Context, policy domain.Policy) (domain.Policy, error)
	Update(ctx context.Context, policy domain.Policy) (domain.Policy, error)
	FindByID(ctx context.Context, id string) (*domain.Policy, error)
	List(ctx context.Context, limit, offset int) ([]domain.Policy, error)
	ListActive(ctx context.Context) ([]domain.Policy, error)
	Disable(ctx context.Context, id string) error
	InsertDecision(ctx context.Context, decision domain.Decision) (domain.Decision, error)
	ListDecisions(ctx context.Context, limit, offset int) ([]domain.Decision, error)
	ListDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]domain.Decision, error)
	InsertTrustScoreResult(ctx context.Context, result domain.TrustScoreResult) (domain.TrustScoreResult, error)
}

type TrustScoreConfig struct {
	BaseScore               int
	LDAPRegistryMatch       int
	RegisteredOwner         int
	KnownDeviceType         int
	DepartmentPresent       int
	DefaultVLANPresent      int
	StableAttachment        int
	LDAPNotFound            int
	UnknownDeviceType       int
	RapidPortMovement       int
	PreviousQuarantine      int
	IPMACAnomaly            int
	PortProfileMismatch     int
	RepeatedEnrichmentError int
}

type Config struct {
	EnforcementEnabled    bool
	DefaultDryRun         bool
	ThresholdAllow        int
	ThresholdMonitor      int
	ThresholdRestricted   int
	ThresholdRegistration int
	TrustScore            TrustScoreConfig
}

type EvaluationInput struct {
	DeviceID              string
	PortEventID           string
	MACAddress            string
	IPAddress             string
	Hostname              string
	VendorClass           string
	SwitchID              string
	SwitchName            string
	SwitchManagementIP    string
	SwitchVendor          string
	PortID                string
	Interface             string
	PortProfile           string
	CurrentVLAN           int
	Site                  string
	Building              string
	Zone                  string
	DeviceType            string
	OperatingSystem       string
	FirstSeenAt           time.Time
	LastSeenAt            time.Time
	KnownDevice           bool
	ManagedDevice         bool
	EnrichmentSource      string
	EnrichmentStatus      string
	RegisteredOwner       string
	OwnerUsername         string
	OwnerDepartment       string
	OwnerRole             string
	AssignedPolicy        string
	RegisteredVendor      string
	DefaultVLANID         int
	LDAPRegistryMatch     bool
	PreviousQuarantine    bool
	PortChangeCount       int
	RapidPortMovement     bool
	IPMACAnomaly          bool
	FailedEnrichmentCount int
	UnknownDevice         bool
	LastPolicyDecision    string
	LastViolation         string
	LastEnforcementAction string
	LastEnforcementStatus string
	AuthenticationMethod  string
	TrustLevel            string
	ObservationSource     string
	SophosUsername        string
}

type EvaluationResult struct {
	Status               string               `json:"status"`
	Action               string               `json:"action"`
	Reason               string               `json:"reason"`
	PolicyID             string               `json:"policy_id"`
	PolicyName           string               `json:"policy_name"`
	DecisionType         string               `json:"decision_type"`
	TargetVLAN           int                  `json:"target_vlan"`
	EnforcementAction    string               `json:"enforcement_action"`
	TrustScore           int                  `json:"trust_score"`
	TrustSignals         []domain.TrustSignal `json:"trust_signals"`
	ReasonCodes          []string             `json:"reason_codes"`
	Explanation          string               `json:"explanation"`
	DryRun               bool                 `json:"dry_run"`
	EvaluationDurationMS int64                `json:"evaluation_duration_ms"`
	MatchedPolicies      []string             `json:"matched_policies"`
	DecisionID           string               `json:"decision_id"`
}

type CreateInput struct {
	Name              string
	Description       string
	Priority          int
	Enabled           bool
	MatchConditions   []domain.Condition
	DecisionType      string
	TargetVLAN        int
	EnforcementAction string
	DryRun            bool
}

type UpdateInput struct {
	Name              *string
	Description       *string
	Priority          *int
	Enabled           *bool
	MatchConditions   []domain.Condition
	DecisionType      *string
	TargetVLAN        *int
	EnforcementAction *string
	DryRun            *bool
}

type Service struct {
	repository Repository
	audit      *auditlogservice.Service
	config     Config
}

func NewService(repository Repository, audit *auditlogservice.Service, cfg Config) *Service {
	cfg = normalizeConfig(cfg)
	return &Service{repository: repository, audit: audit, config: cfg}
}

func (s *Service) EnforcementEnabled() bool {
	if s == nil {
		return false
	}
	return s.config.EnforcementEnabled && !s.config.DefaultDryRun
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]domain.Policy, error) {
	if s == nil || s.repository == nil {
		return []domain.Policy{}, nil
	}
	return s.repository.List(ctx, limit, offset)
}
func (s *Service) ListActive(ctx context.Context) ([]domain.Policy, error) {
	if s == nil || s.repository == nil {
		return []domain.Policy{}, nil
	}
	return s.repository.ListActive(ctx)
}
func (s *Service) FindByID(ctx context.Context, id string) (*domain.Policy, error) {
	if s == nil || s.repository == nil {
		return nil, nil
	}
	return s.repository.FindByID(ctx, strings.TrimSpace(id))
}
func (s *Service) ListDecisions(ctx context.Context, limit, offset int) ([]domain.Decision, error) {
	if s == nil || s.repository == nil {
		return []domain.Decision{}, nil
	}
	return s.repository.ListDecisions(ctx, limit, offset)
}
func (s *Service) ListDecisionsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]domain.Decision, error) {
	if s == nil || s.repository == nil {
		return []domain.Decision{}, nil
	}
	return s.repository.ListDecisionsByDevice(ctx, strings.TrimSpace(deviceID), limit, offset)
}
func (s *Service) Disable(ctx context.Context, id string) error {
	if s == nil || s.repository == nil {
		return nil
	}
	return s.repository.Disable(ctx, strings.TrimSpace(id))
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domain.Policy, error) {
	now := time.Now().UTC()
	item := domain.Policy{ID: uuid.NewString(), Name: strings.TrimSpace(input.Name), Description: strings.TrimSpace(input.Description), Type: "access", Action: legacyActionForDecision(input.DecisionType), MatchField: firstConditionField(input.MatchConditions), MatchOperator: firstConditionOperator(input.MatchConditions), MatchValue: firstConditionValue(input.MatchConditions), Priority: input.Priority, Status: policyStatus(input.Enabled), Enabled: input.Enabled, MatchConditions: normalizeConditions(input.MatchConditions), DecisionType: normalizeDecisionType(input.DecisionType), TargetVLAN: input.TargetVLAN, EnforcementAction: normalizeEnforcementAction(input.EnforcementAction, input.DecisionType), DryRun: input.DryRun, CreatedAt: now, UpdatedAt: now}
	return s.repository.Insert(ctx, item)
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (domain.Policy, error) {
	current, err := s.FindByID(ctx, id)
	if err != nil {
		return domain.Policy{}, err
	}
	if current == nil {
		return domain.Policy{}, fmt.Errorf("policy not found")
	}
	item := *current
	if input.Name != nil {
		item.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		item.Description = strings.TrimSpace(*input.Description)
	}
	if input.Priority != nil {
		item.Priority = *input.Priority
	}
	if input.Enabled != nil {
		item.Enabled = *input.Enabled
		item.Status = policyStatus(*input.Enabled)
	}
	if input.MatchConditions != nil {
		item.MatchConditions = normalizeConditions(input.MatchConditions)
		item.MatchField = firstConditionField(item.MatchConditions)
		item.MatchOperator = firstConditionOperator(item.MatchConditions)
		item.MatchValue = firstConditionValue(item.MatchConditions)
	}
	if input.DecisionType != nil {
		item.DecisionType = normalizeDecisionType(*input.DecisionType)
		item.Action = legacyActionForDecision(item.DecisionType)
	}
	if input.TargetVLAN != nil {
		item.TargetVLAN = *input.TargetVLAN
	}
	if input.EnforcementAction != nil {
		item.EnforcementAction = normalizeEnforcementAction(*input.EnforcementAction, item.DecisionType)
	}
	if input.DryRun != nil {
		item.DryRun = *input.DryRun
	}
	item.UpdatedAt = time.Now().UTC()
	return s.repository.Update(ctx, item)
}
