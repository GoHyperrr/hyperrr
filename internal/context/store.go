package context

import (
	"context"
	"encoding/json"
	"time"

	"github.com/GoHyperrr/hyperrr/pkg/db"
)

// LineageModel is the DB representation of a workflow lineage.
type LineageModel struct {
	ID        string    `gorm:"primaryKey"`
	Name      string    `gorm:"index"`
	Version   string
	State     string    `gorm:"index"`
	StartedAt time.Time
	EndedAt   *time.Time
	Error     string
	Steps     string `gorm:"type:text"` // JSON
	Events    string `gorm:"type:text"` // JSON
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LineageStore handles persistence for lineages.
type LineageStore struct {
	db *db.DB
}

func NewLineageStore(database *db.DB) *LineageStore {
	return &LineageStore{db: database}
}

func (s *LineageStore) Save(ctx context.Context, l *Lineage) error {
	stepsJSON, _ := json.Marshal(l.Steps)
	eventsJSON, _ := json.Marshal(l.Events)

	m := &LineageModel{
		ID:        l.ID,
		Name:      l.Name,
		Version:   l.Version,
		State:     l.State,
		StartedAt: l.StartedAt,
		EndedAt:   l.EndedAt,
		Error:     l.Error,
		Steps:     string(stepsJSON),
		Events:    string(eventsJSON),
	}

	return s.db.WithContext(ctx).Save(m).Error
}

func (s *LineageStore) Get(ctx context.Context, id string) (*Lineage, error) {
	var m LineageModel
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return s.toDomain(&m), nil
}

func (s *LineageStore) List(ctx context.Context) ([]*Lineage, error) {
	var models []*LineageModel
	if err := s.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}
	res := make([]*Lineage, 0, len(models))
	for _, m := range models {
		res = append(res, s.toDomain(m))
	}
	return res, nil
}

func (s *LineageStore) toDomain(m *LineageModel) *Lineage {
	var steps []*StepLineage
	json.Unmarshal([]byte(m.Steps), &steps)
	
	return &Lineage{
		ID:        m.ID,
		Name:      m.Name,
		Version:   m.Version,
		State:     m.State,
		StartedAt: m.StartedAt,
		EndedAt:   m.EndedAt,
		Error:     m.Error,
		Steps:     steps,
	}
}
