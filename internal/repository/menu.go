package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"menu-tree-backend/internal/model"
)

const menuColumns = "id, name, parent_id, depth, position, created_at, updated_at"

const isInSubtreeQuery = `
	WITH RECURSIVE subtree AS (
		SELECT id FROM menus WHERE id = $1
		UNION ALL
		SELECT m.id FROM menus m JOIN subtree s ON m.parent_id = s.id
	)
	SELECT EXISTS (SELECT 1 FROM subtree WHERE id = $2)`

const shiftDescendantDepthQuery = `
	WITH RECURSIVE sub AS (
		SELECT id FROM menus WHERE parent_id = $1
		UNION ALL
		SELECT m.id FROM menus m JOIN sub s ON m.parent_id = s.id
	)
	UPDATE menus SET depth = depth + $2 WHERE id IN (SELECT id FROM sub)`

type MenuRepository struct {
	db *pgxpool.Pool
}

func NewMenuRepository(db *pgxpool.Pool) *MenuRepository {
	return &MenuRepository{db: db}
}

// sameUUID compares two UUID pointers; nil equals nil.
func sameUUID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// scanMenu
func scanMenu(row pgx.Row) (*model.Menu, error) {
	var m model.Menu
	err := row.Scan(&m.ID, &m.Name, &m.ParentID, &m.Depth, &m.Position, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// Create menu
func (r *MenuRepository) Create(ctx context.Context, name string, parentID *uuid.UUID, depth int) (*model.Menu, error) {
	query := `
		INSERT INTO menus (name, parent_id, depth, position)
		VALUES (
			$1, $2::uuid, $3,
			(SELECT COALESCE(MAX(position) + 1, 0) FROM menus WHERE parent_id IS NOT DISTINCT FROM $2::uuid)
		)
		RETURNING ` + menuColumns
	return scanMenu(r.db.QueryRow(ctx, query, name, parentID, depth))
}

func (r *MenuRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Menu, error) {
	query := `SELECT ` + menuColumns + ` FROM menus WHERE id = $1`
	return scanMenu(r.db.QueryRow(ctx, query, id))
}

func (r *MenuRepository) GetAll(ctx context.Context) ([]model.Menu, error) {
	query := `SELECT ` + menuColumns + ` FROM menus ORDER BY position, created_at`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	menus := []model.Menu{}
	for rows.Next() {
		m, err := scanMenu(rows)
		if err != nil {
			return nil, err
		}
		menus = append(menus, *m)
	}
	return menus, rows.Err()
}

func (r *MenuRepository) Update(ctx context.Context, id uuid.UUID, name string) (*model.Menu, error) {
	query := `UPDATE menus SET name = $2, updated_at = now() WHERE id = $1 RETURNING ` + menuColumns
	return scanMenu(r.db.QueryRow(ctx, query, id, name))
}

func (r *MenuRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM menus WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

// Move moves a menu under newParentID (nil = root) at position (nil = last).
// Everything runs in one transaction.
func (r *MenuRepository) Move(ctx context.Context, id uuid.UUID, newParentID *uuid.UUID, position *int) (*model.Menu, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(42)`); err != nil {
		return nil, err
	}

	current, err := scanMenu(tx.QueryRow(ctx, `SELECT `+menuColumns+` FROM menus WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}

	sameParent := sameUUID(current.ParentID, newParentID)

	newDepth := 0
	if newParentID != nil {
		parent, err := scanMenu(tx.QueryRow(ctx, `SELECT `+menuColumns+` FROM menus WHERE id = $1`, *newParentID))
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return nil, model.ErrParentNotFound
			}
			return nil, err
		}

		if !sameParent {
			var inSubtree bool
			if err := tx.QueryRow(ctx, isInSubtreeQuery, id, *newParentID).Scan(&inSubtree); err != nil {
				return nil, err
			}
			if inSubtree {
				return nil, model.ErrCycle
			}
		}
		newDepth = parent.Depth + 1
	}

	if _, err := tx.Exec(ctx,
		`UPDATE menus SET position = position - 1
		 WHERE parent_id IS NOT DISTINCT FROM $1::uuid AND position > $2`,
		current.ParentID, current.Position); err != nil {
		return nil, err
	}

	var siblings int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM menus WHERE parent_id IS NOT DISTINCT FROM $1::uuid AND id <> $2`,
		newParentID, id).Scan(&siblings); err != nil {
		return nil, err
	}
	newPos := siblings
	if position != nil && *position < siblings {
		newPos = *position
	}

	if _, err := tx.Exec(ctx,
		`UPDATE menus SET position = position + 1
		 WHERE parent_id IS NOT DISTINCT FROM $1::uuid AND position >= $2 AND id <> $3`,
		newParentID, newPos, id); err != nil {
		return nil, err
	}

	moved, err := scanMenu(tx.QueryRow(ctx,
		`UPDATE menus SET parent_id = $2::uuid, position = $3, depth = $4, updated_at = now()
		 WHERE id = $1 RETURNING `+menuColumns,
		id, newParentID, newPos, newDepth))
	if err != nil {
		return nil, err
	}

	if delta := newDepth - current.Depth; delta != 0 {
		if _, err := tx.Exec(ctx, shiftDescendantDepthQuery, id, delta); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return moved, nil
}
