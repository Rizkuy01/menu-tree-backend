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

type MenuRepository struct {
	db *pgxpool.Pool
}

func NewMenuRepository(db *pgxpool.Pool) *MenuRepository {
	return &MenuRepository{db: db}
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
