package policy

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "nac/internal/domain/policy"
)

type Repository interface {
	Insert(ctx context.Context, policy domain.Policy) (domain.Policy, error)
	ListActive(ctx context.Context) ([]domain.Policy, error)
	Disable(ctx context.Context, id string) error
}

type EvaluationInput struct {
	MACAddress           string
	Hostname             string
	VendorClass          string
	SwitchID             string
	SwitchName           string
	Interface            string
	DeviceType           string
	AuthenticationMethod string
	TrustLevel           string
	ObservationSource    string
	SophosUsername       string
}

type EvaluationResult struct {
	Status string
	Action string
	Reason string
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

type CreateInput struct {
	Name          string
	Description   string
	Action        string
	MatchField    string
	MatchOperator string
	MatchValue    string
	Priority      int
}

func (s *Service) ListActive(ctx context.Context) ([]domain.Policy, error) {
	return s.repository.ListActive(ctx)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (domain.Policy, error) {
	now := time.Now().UTC()
	policy := domain.Policy{
		ID:            uuid.NewString(),
		Name:          strings.TrimSpace(input.Name),
		Description:   strings.TrimSpace(input.Description),
		Type:          "classification",
		Action:        strings.TrimSpace(input.Action),
		MatchField:    strings.TrimSpace(input.MatchField),
		MatchOperator: strings.TrimSpace(input.MatchOperator),
		MatchValue:    strings.TrimSpace(input.MatchValue),
		Priority:      input.Priority,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	return s.repository.Insert(ctx, policy)
}

func (s *Service) Disable(ctx context.Context, id string) error {
	return s.repository.Disable(ctx, strings.TrimSpace(id))
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	policies, err := s.repository.ListActive(ctx)
	if err != nil {
		return err
	}
	if len(policies) > 0 {
		return nil
	}

	now := time.Now().UTC()
	defaults := []domain.Policy{
		{
			ID:            uuid.NewString(),
			Name:          "Hostname Missing",
			Description:   "Hostname gelmeyen cihazlari observed olarak isaretle",
			Type:          "classification",
			Action:        "observed",
			MatchField:    "hostname",
			MatchOperator: "empty",
			MatchValue:    "",
			Priority:      100,
			Status:        "active",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            uuid.NewString(),
			Name:          "Known Hostname Prefix",
			Description:   "Kurumsal hostname desenlerini active say",
			Type:          "classification",
			Action:        "active",
			MatchField:    "hostname",
			MatchOperator: "contains",
			MatchValue:    "LAB",
			Priority:      80,
			Status:        "active",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            uuid.NewString(),
			Name:          "Default Unknown",
			Description:   "Varsayilan kural",
			Type:          "classification",
			Action:        "unknown",
			MatchField:    "mac_address",
			MatchOperator: "any",
			MatchValue:    "*",
			Priority:      1,
			Status:        "active",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	for _, policy := range defaults {
		if _, err := s.repository.Insert(ctx, policy); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) Evaluate(ctx context.Context, input EvaluationInput) (EvaluationResult, error) {
	policies, err := s.repository.ListActive(ctx)
	if err != nil {
		return EvaluationResult{}, err
	}

	fields := map[string]string{
		"mac_address":           strings.TrimSpace(input.MACAddress),
		"hostname":              strings.TrimSpace(input.Hostname),
		"vendor_class":          strings.TrimSpace(input.VendorClass),
		"switch_id":             strings.TrimSpace(input.SwitchID),
		"switch_name":           strings.TrimSpace(input.SwitchName),
		"interface":             strings.TrimSpace(input.Interface),
		"device_type":           strings.TrimSpace(input.DeviceType),
		"authentication_method": strings.TrimSpace(input.AuthenticationMethod),
		"trust_level":           strings.TrimSpace(input.TrustLevel),
		"observation_source":    strings.TrimSpace(input.ObservationSource),
		"sophos_username":       strings.TrimSpace(input.SophosUsername),
	}

	for _, policy := range policies {
		value := fields[policy.MatchField]
		if !matches(policy.MatchOperator, value, policy.MatchValue) {
			continue
		}

		return EvaluationResult{
			Status: policy.Action,
			Action: policy.Action,
			Reason: policy.Name,
		}, nil
	}

	return EvaluationResult{Status: "unknown", Action: "unknown", Reason: "No policy matched"}, nil
}

func matches(operator, value, matchValue string) bool {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "any":
		return true
	case "empty":
		return strings.TrimSpace(value) == ""
	case "equals":
		return strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(matchValue))
	case "contains":
		return strings.Contains(strings.ToLower(value), strings.ToLower(strings.TrimSpace(matchValue)))
	default:
		return false
	}
}
