# TASK: Implement `move` and `reorder` endpoints

> Read `AGENTS.md` first for project context, conventions, and pitfalls.

## Goal

Add two bonus endpoints to the Go backend:

- `PATCH /api/menus/:id/move` — move a menu (with its whole subtree) under another parent, or to root.
- `PATCH /api/menus/:id/reorder` — change a menu's position among its siblings.

## Design (do not deviate without reason)

**One core function, `Move`, handles both.** `reorder` is just `move` to the *same* parent with a new position.

`Move` runs in **one transaction** (all-or-nothing):

1. Take an advisory lock (`pg_advisory_xact_lock(42)`) so concurrent structure changes cannot create duplicate positions.
2. Load the menu being moved; load the target parent (if any).
3. **Cycle check** (recursive CTE): the target parent must not be the menu itself or any of its descendants. Skipped if the parent does not change.
4. Close the gap in the old parent (siblings with greater position get `position - 1`).
5. Compute the target position among the new siblings (excluding itself). Omitted = last. Larger than count = clamped to last.
6. Open a gap in the new parent (siblings with `position >= newPos` get `+1`, excluding itself).
7. Update the menu: `parent_id`, `position`, `depth`.
8. If depth changed, add the same delta to **all descendants'** depth.

API rules:
- `position` is 0-based. `parent_id: null` in `move` means root.
- `Position` is a `*int` in DTOs so "not sent" (nil) differs from `0`.
- Cycle → `409 INVALID_MOVE`. Missing menu → `404 NOT_FOUND`. Missing target parent → `404 PARENT_NOT_FOUND`.
  Negative/missing position → `400 VALIDATION_ERROR`.

## Steps

### Step 1 — Sentinel errors

In `internal/model/errors.go`, inside the existing `var (...)` block add:

```go
	ErrCycle           = errors.New("cannot move a menu into itself or its own descendant")
	ErrInvalidPosition = errors.New("position must be zero or greater")
```

### Step 2 — DTOs

Append to `internal/dto/menu.go`:

```go
type MoveMenuRequest struct {
	ParentID *uuid.UUID `json:"parent_id" swaggertype:"string" format:"uuid" example:"56320ee9-6af6-11ed-a7ba-f220afe5e4a9"`
	Position *int       `json:"position" binding:"omitempty,min=0" example:"0"`
}

type ReorderMenuRequest struct {
	Position *int `json:"position" binding:"required,min=0" example:"0"`
}
```

### Step 3 — Repository

Append to `internal/repository/menu.go` (existing imports are sufficient):

```go
// sameUUID compares two UUID pointers; nil equals nil.
func sameUUID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// isInSubtreeQuery: is $2 equal to $1 or one of its descendants?
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

// Move moves a menu under newParentID (nil = root) at position (nil = last).
// Everything runs in one transaction.
func (r *MenuRepository) Move(ctx context.Context, id uuid.UUID, newParentID *uuid.UUID, position *int) (*model.Menu, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// Safe to call after Commit (error ignored); guarantees rollback on early return.
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize tree-structure changes; released automatically when the tx ends.
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

	// 1. Close the gap in the old parent.
	if _, err := tx.Exec(ctx,
		`UPDATE menus SET position = position - 1
		 WHERE parent_id IS NOT DISTINCT FROM $1::uuid AND position > $2`,
		current.ParentID, current.Position); err != nil {
		return nil, err
	}

	// 2. Target position among the new siblings (excluding itself).
	var siblings int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM menus WHERE parent_id IS NOT DISTINCT FROM $1::uuid AND id <> $2`,
		newParentID, id).Scan(&siblings); err != nil {
		return nil, err
	}
	newPos := siblings // default: last
	if position != nil && *position < siblings {
		newPos = *position
	}

	// 3. Open a gap in the new parent.
	if _, err := tx.Exec(ctx,
		`UPDATE menus SET position = position + 1
		 WHERE parent_id IS NOT DISTINCT FROM $1::uuid AND position >= $2 AND id <> $3`,
		newParentID, newPos, id); err != nil {
		return nil, err
	}

	// 4. Move the menu itself.
	moved, err := scanMenu(tx.QueryRow(ctx,
		`UPDATE menus SET parent_id = $2::uuid, position = $3, depth = $4, updated_at = now()
		 WHERE id = $1 RETURNING `+menuColumns,
		id, newParentID, newPos, newDepth))
	if err != nil {
		return nil, err
	}

	// 5. Shift descendants' depth if this menu's depth changed.
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
```

### Step 4 — Service

In `internal/service/menu.go`:

1. Add this line to the `MenuRepository` interface:

```go
	Move(ctx context.Context, id uuid.UUID, newParentID *uuid.UUID, position *int) (*model.Menu, error)
```

2. Append these methods:

```go
func (s *MenuService) Move(ctx context.Context, id uuid.UUID, parentID *uuid.UUID, position *int) (*model.Menu, error) {
	if position != nil && *position < 0 {
		return nil, model.ErrInvalidPosition
	}
	return s.repo.Move(ctx, id, parentID, position)
}

// Reorder = Move to the same parent with a new position.
func (s *MenuService) Reorder(ctx context.Context, id uuid.UUID, position int) (*model.Menu, error) {
	if position < 0 {
		return nil, model.ErrInvalidPosition
	}
	menu, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.repo.Move(ctx, id, menu.ParentID, &position)
}
```

### Step 5 — Map errors to HTTP

In `internal/handler/response.go`, inside the `switch` of `handleServiceError`, add before `default`:

```go
	case errors.Is(err, model.ErrCycle):
		respondError(c, http.StatusConflict, "INVALID_MOVE", err.Error())
	case errors.Is(err, model.ErrInvalidPosition):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
```

### Step 6 — Handlers (with swagger annotations)

Append to `internal/handler/menu.go`:

```go
// Move godoc
// @Summary      Move a menu to another parent
// @Description  Moves a menu (with all its descendants) under a different parent. Use parent_id null for root. Position is 0-based; omit it to append at the end. Rejects moving a menu into itself or its own descendants.
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        id    path      string               true  "Menu ID (UUID)"  format(uuid)
// @Param        body  body      dto.MoveMenuRequest  true  "Target parent and position"
// @Success      200   {object}  model.Menu
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse  "Menu or parent not found"
// @Failure      409   {object}  ErrorResponse  "Invalid move (cycle)"
// @Failure      500   {object}  ErrorResponse
// @Router       /menus/{id}/move [patch]
func (h *MenuHandler) Move(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.MoveMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body: parent_id must be a UUID or null, position must be >= 0")
		return
	}
	menu, err := h.svc.Move(c.Request.Context(), id, req.ParentID, req.Position)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, menu)
}

// Reorder godoc
// @Summary      Reorder a menu within its level
// @Description  Changes the position of a menu among its siblings (same parent). Position is 0-based and clamped to the last index.
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        id    path      string                  true  "Menu ID (UUID)"  format(uuid)
// @Param        body  body      dto.ReorderMenuRequest  true  "New position"
// @Success      200   {object}  model.Menu
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /menus/{id}/reorder [patch]
func (h *MenuHandler) Reorder(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.ReorderMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body: position is required and must be >= 0")
		return
	}
	menu, err := h.svc.Reorder(c.Request.Context(), id, *req.Position)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, menu)
}
```

### Step 7 — Routes

In `internal/router/router.go`, inside the `menus` group add:

```go
		menus.PATCH("/:id/move", menuHandler.Move)
		menus.PATCH("/:id/reorder", menuHandler.Reorder)
```

### Step 8 — Regenerate docs, build, run

```bash
swag init -g cmd/api/main.go -o docs
gofmt -l .
go vet ./...
go build ./...
go run ./cmd/api
```

## Verification script

Run in a second Git Bash terminal while the server is running (needs the `DATABASE_URL` DB to be reachable).
It creates `Root → A, B, C` and `A → G`, then exercises every scenario.

```bash
API=http://localhost:8080/api/menus
H='Content-Type: application/json'
mk() { curl -s -X POST $API -H "$H" -d "$1" | sed -E 's/.*"id":"([^"]+)".*/\1/'; }

R=$(mk '{"name":"Root"}')
A=$(mk "{\"name\":\"A\",\"parent_id\":\"$R\"}")
B=$(mk "{\"name\":\"B\",\"parent_id\":\"$R\"}")
C=$(mk "{\"name\":\"C\",\"parent_id\":\"$R\"}")
G=$(mk "{\"name\":\"G\",\"parent_id\":\"$A\"}")

# 1. reorder C to position 0  -> expect order C(0), A(1), B(2)
curl -s -X PATCH $API/$C/reorder -H "$H" -d '{"position":0}'; echo
# 2. reorder beyond range      -> clamped to last: A(0), B(1), C(2)
curl -s -X PATCH $API/$C/reorder -H "$H" -d '{"position":99}'; echo
# 3. move G under B            -> G.parent_id = B, A has no children
curl -s -X PATCH $API/$G/move -H "$H" -d "{\"parent_id\":\"$B\"}"; echo
# 4. move B (with G) to root   -> B.depth 0, G.depth 1
curl -s -X PATCH $API/$B/move -H "$H" -d '{"parent_id":null}'; echo
curl -s $API; echo
# 5. cycle: B into its child G, and B into itself -> 409 INVALID_MOVE (both)
curl -s -X PATCH $API/$B/move -H "$H" -d "{\"parent_id\":\"$G\"}"; echo
curl -s -X PATCH $API/$B/move -H "$H" -d "{\"parent_id\":\"$B\"}"; echo
# 6. errors
curl -s -X PATCH $API/$A/move -H "$H" -d '{"parent_id":"00000000-0000-0000-0000-000000000000"}'; echo  # 404 PARENT_NOT_FOUND
curl -s -X PATCH $API/00000000-0000-0000-0000-000000000000/reorder -H "$H" -d '{"position":0}'; echo   # 404 NOT_FOUND
curl -s -X PATCH $API/$A/reorder -H "$H" -d '{"position":-1}'; echo                                     # 400 VALIDATION_ERROR
curl -s -X PATCH $API/$A/reorder -H "$H" -d '{}'; echo                                                  # 400 VALIDATION_ERROR

# cleanup
curl -s -X DELETE $API/$R; curl -s -X DELETE $API/$B
```

## Acceptance criteria

- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` are clean.
- [ ] After any operation, siblings at each level have contiguous positions `0..n-1` with no duplicates.
- [ ] Moving a subtree updates the depth of **all** descendants.
- [ ] Moving into itself or a descendant returns `409` with code `INVALID_MOVE` and changes nothing.
- [ ] `parent_id: null` moves to root; omitted `position` appends last; oversized `position` clamps.
- [ ] All error cases in the table in `AGENTS.md` return the documented status + code.
- [ ] Swagger UI (`/swagger/index.html`) shows both new PATCH endpoints with request bodies.
- [ ] No secrets committed; `docs/` regenerated, not hand-edited.

## Finish

```bash
git add .
git commit -m "feat: move and reorder endpoints with cycle detection"
```

Then report back: what was changed, which verification steps were actually run and their results,
and anything that could not be verified.

## Suggested next tasks (separate files / separate sessions)

1. Unit tests for the service layer (mock `MenuRepository`): `buildTree`, depth on create, name validation, `Reorder`/`Move` input rules.
2. Frontend (Next.js + TypeScript + Tailwind + Zustand) following the design spec in `AGENTS.md` section 8.
3. README, then optional Docker files, then screenshots/demo video.
