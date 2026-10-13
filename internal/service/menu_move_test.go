package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"menu-tree-backend/internal/model"
)

type stubMenuRepo struct {
	menus map[uuid.UUID]model.Menu
}

func (s *stubMenuRepo) Create(ctx context.Context, name string, parentID *uuid.UUID, depth int) (*model.Menu, error) {
	id := uuid.New()
	menu := model.Menu{
		ID:        id,
		Name:      name,
		ParentID:  parentID,
		Depth:     depth,
		Position:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.menus[id] = menu
	return &menu, nil
}

func (s *stubMenuRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Menu, error) {
	m, ok := s.menus[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	return &m, nil
}

func (s *stubMenuRepo) GetAll(ctx context.Context) ([]model.Menu, error) {
	menus := make([]model.Menu, 0, len(s.menus))
	for _, m := range s.menus {
		menus = append(menus, m)
	}
	return menus, nil
}

func (s *stubMenuRepo) Update(ctx context.Context, id uuid.UUID, name string) (*model.Menu, error) {
	m, ok := s.menus[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	m.Name = name
	m.UpdatedAt = time.Now()
	s.menus[id] = m
	return &m, nil
}

func (s *stubMenuRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := s.menus[id]; !ok {
		return model.ErrNotFound
	}
	delete(s.menus, id)
	return nil
}

func (s *stubMenuRepo) Move(ctx context.Context, id uuid.UUID, newParentID *uuid.UUID, position *int) (*model.Menu, error) {
	menu, ok := s.menus[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	if position != nil && *position < 0 {
		return nil, model.ErrInvalidPosition
	}
	menu.ParentID = newParentID
	if position != nil {
		menu.Position = *position
	}
	s.menus[id] = menu
	return &menu, nil
}

func TestServiceMoveRejectsNegativePosition(t *testing.T) {
	repo := &stubMenuRepo{menus: map[uuid.UUID]model.Menu{}}
	svc := NewMenuService(repo)

	_, err := svc.Move(context.Background(), uuid.New(), nil, ptrInt(-1))
	if err == nil || err != model.ErrInvalidPosition {
		t.Fatalf("expected ErrInvalidPosition, got %v", err)
	}
}

func TestServiceReorderRejectsNegativePosition(t *testing.T) {
	repo := &stubMenuRepo{menus: map[uuid.UUID]model.Menu{}}
	svc := NewMenuService(repo)

	_, err := svc.Reorder(context.Background(), uuid.New(), -1)
	if err == nil || err != model.ErrInvalidPosition {
		t.Fatalf("expected ErrInvalidPosition, got %v", err)
	}
}

func ptrInt(v int) *int { return &v }
