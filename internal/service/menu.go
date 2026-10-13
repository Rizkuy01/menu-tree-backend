package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"menu-tree-backend/internal/model"
)

// MenuRepository interface
type MenuRepository interface {
	Create(ctx context.Context, name string, parentID *uuid.UUID, depth int) (*model.Menu, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Menu, error)
	GetAll(ctx context.Context) ([]model.Menu, error)
	Update(ctx context.Context, id uuid.UUID, name string) (*model.Menu, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type MenuService struct {
	repo MenuRepository
}

func NewMenuService(repo MenuRepository) *MenuService {
	return &MenuService{repo: repo}
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", model.ErrInvalidName
	}
	return name, nil
}

func (s *MenuService) Create(ctx context.Context, name string, parentID *uuid.UUID) (*model.Menu, error) {
	name, err := normalizeName(name)
	if err != nil {
		return nil, err
	}

	depth := 0
	if parentID != nil {
		parent, err := s.repo.GetByID(ctx, *parentID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return nil, model.ErrParentNotFound
			}
			return nil, err
		}
		depth = parent.Depth + 1
	}

	return s.repo.Create(ctx, name, parentID, depth)
}

func (s *MenuService) GetByID(ctx context.Context, id uuid.UUID) (*model.Menu, error) {
	menu, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if menu.ParentID != nil {
		parent, err := s.repo.GetByID(ctx, *menu.ParentID)
		if err != nil {
			return nil, err
		}
		menu.ParentName = &parent.Name
	}
	return menu, nil
}

func (s *MenuService) Update(ctx context.Context, id uuid.UUID, name string) (*model.Menu, error) {
	name, err := normalizeName(name)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, name)
}

func (s *MenuService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// GetTree
func (s *MenuService) GetTree(ctx context.Context) ([]*model.MenuNode, error) {
	menus, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return buildTree(menus), nil
}

func buildTree(menus []model.Menu) []*model.MenuNode {
	nodes := make(map[uuid.UUID]*model.MenuNode, len(menus))
	for _, m := range menus {
		nodes[m.ID] = &model.MenuNode{
			ID:       m.ID,
			Name:     m.Name,
			ParentID: m.ParentID,
			Depth:    m.Depth,
			Position: m.Position,
			Children: []*model.MenuNode{},
		}
	}

	roots := []*model.MenuNode{}
	for _, m := range menus {
		node := nodes[m.ID]
		if m.ParentID == nil {
			roots = append(roots, node)
			continue
		}
		if parent, ok := nodes[*m.ParentID]; ok {
			parent.Children = append(parent.Children, node)
		}
	}
	return roots
}
