package ctxengine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GoHyperrr/hyperrr/api/graph/model"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Ensure Module implements registry.GraphQLProvider at compile time.
var _ registry.GraphQLProvider = (*Module)(nil)

func (m *Module) Queries() map[string]any {
	return map[string]any{
		"getWorkflowLineage": m.GetWorkflowLineage,
		"listLineages":       m.ListLineages,
		"health":             m.HealthQuery,
	}
}

func (m *Module) Mutations() map[string]any {
	return map[string]any{
		"_health": m.HealthMutation,
	}
}

func (m *Module) FieldResolvers() map[string]any {
	return map[string]any{
		"WorkflowLineage.events":          m.Events,
		"WorkflowLineage.relatedLineages": m.RelatedLineages,
	}
}

func (m *Module) HealthQuery(ctx context.Context) (string, error) {
	return "OK", nil
}

func (m *Module) HealthMutation(ctx context.Context) (string, error) {
	return "", fmt.Errorf("not implemented: Health - _health")
}

func (m *Module) GetWorkflowLineage(ctx context.Context, id string) (*model.WorkflowLineage, error) {
	lineage, err := m.projector.GetLineage(id)
	if err != nil {
		return nil, err
	}

	return mapToModel(lineage), nil
}

func (m *Module) ListLineages(ctx context.Context) ([]*model.WorkflowLineage, error) {
	lineages := m.projector.ListLineages()
	res := make([]*model.WorkflowLineage, 0, len(lineages))
	for _, l := range lineages {
		conc, ok := l.(*Lineage)
		if ok {
			res = append(res, mapToModel(conc))
		}
	}
	return res, nil
}

func (m *Module) Events(ctx context.Context, obj *model.WorkflowLineage) ([]*model.Event, error) {
	lineage, err := m.projector.GetLineage(obj.ID)
	if err != nil {
		return nil, err
	}
	res := mapToModel(lineage)
	return res.Events, nil
}

func (m *Module) RelatedLineages(ctx context.Context, obj *model.WorkflowLineage) ([]*model.WorkflowLineage, error) {
	lineages, err := m.projector.GetRelatedLineages(ctx, obj.ID)
	if err != nil {
		return nil, err
	}

	res := make([]*model.WorkflowLineage, 0, len(lineages))
	for _, l := range lineages {
		res = append(res, mapToModel(l))
	}
	return res, nil
}

func mapToModel(l *Lineage) *model.WorkflowLineage {
	res := &model.WorkflowLineage{
		ID:        l.ID,
		Name:      l.Name,
		Version:   l.Version,
		State:     l.State,
		StartedAt: l.StartedAt,
		EndedAt:   l.EndedAt,
	}

	if l.Error != "" {
		res.Error = &l.Error
	}

	for _, s := range l.Steps {
		resStep := &model.StepExecution{
			StepID:    s.StepID,
			State:     s.State,
			StartedAt: s.StartedAt,
			EndedAt:   s.EndedAt,
			Attempts:  s.Attempts,
		}
		if s.Error != "" {
			resStep.Error = &s.Error
		}
		res.Steps = append(res.Steps, resStep)
	}

	for _, e := range l.Events {
		payloadJSON, _ := json.Marshal(e.Payload)
		payloadStr := string(payloadJSON)
		res.Events = append(res.Events, &model.Event{
			ID:        e.ID,
			Type:      e.Type,
			Timestamp: e.Timestamp,
			Payload:   &payloadStr,
		})
	}

	return res
}
