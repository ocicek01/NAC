package macobservation

import "context"

type Repository interface {
	Insert(ctx context.Context, observation Observation) (Observation, error)
	ListRecent(ctx context.Context, limit int) ([]Observation, error)
}
