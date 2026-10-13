package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"menu-tree-backend/internal/dto"
	"menu-tree-backend/internal/service"
)

type MenuHandler struct {
	svc *service.MenuService
}

func NewMenuHandler(svc *service.MenuService) *MenuHandler {
	return &MenuHandler{svc: svc}
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID")
		return uuid.Nil, false
	}
	return id, true
}

// GetAll godoc
// @Summary      Get menu tree
// @Description  Returns all root menus with their descendants nested in "children".
// @Tags         menus
// @Produce      json
// @Success      200  {array}   model.MenuNode
// @Failure      500  {object}  ErrorResponse
// @Router       /menus [get]
func (h *MenuHandler) GetAll(c *gin.Context) {
	tree, err := h.svc.GetTree(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, tree)
}

// GetByID godoc
// @Summary      Get a single menu
// @Description  Returns one menu item, including parent_name when it has a parent.
// @Tags         menus
// @Produce      json
// @Param        id   path      string  true  "Menu ID (UUID)"  format(uuid)
// @Success      200  {object}  model.Menu
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /menus/{id} [get]
func (h *MenuHandler) GetByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	menu, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, menu)
}

// Create godoc
// @Summary      Create a menu
// @Description  Creates a menu item. Omit parent_id to create a root menu. Depth is calculated from the parent.
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateMenuRequest  true  "Menu data"
// @Success      201   {object}  model.Menu
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse  "Parent menu not found"
// @Failure      500   {object}  ErrorResponse
// @Router       /menus [post]
func (h *MenuHandler) Create(c *gin.Context) {
	var req dto.CreateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body: name is required (max 255 chars), parent_id must be a UUID")
		return
	}
	menu, err := h.svc.Create(c.Request.Context(), req.Name, req.ParentID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, menu)
}

// Update godoc
// @Summary      Update a menu
// @Description  Updates the name of a menu item.
// @Tags         menus
// @Accept       json
// @Produce      json
// @Param        id    path      string                 true  "Menu ID (UUID)"  format(uuid)
// @Param        body  body      dto.UpdateMenuRequest  true  "New menu data"
// @Success      200   {object}  model.Menu
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /menus/{id} [put]
func (h *MenuHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.UpdateMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body: name is required (max 255 chars)")
		return
	}
	menu, err := h.svc.Update(c.Request.Context(), id, req.Name)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, menu)
}

// Delete godoc
// @Summary      Delete a menu
// @Description  Deletes a menu item and all of its descendants.
// @Tags         menus
// @Param        id   path  string  true  "Menu ID (UUID)"  format(uuid)
// @Success      204  "No Content"
// @Failure      400  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /menus/{id} [delete]
func (h *MenuHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		handleServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

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
