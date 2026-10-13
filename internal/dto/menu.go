package dto

import "github.com/google/uuid"

type CreateMenuRequest struct {
	Name     string     `json:"name" binding:"required,max=255" example:"System Code"`
	ParentID *uuid.UUID `json:"parent_id" swaggertype:"string" format:"uuid" example:"56320ee9-6af6-11ed-a7ba-f220afe5e4a9"`
}

type UpdateMenuRequest struct {
	Name string `json:"name" binding:"required,max=255" example:"System Code"`
}

type MoveMenuRequest struct {
	ParentID *uuid.UUID `json:"parent_id" swaggertype:"string" format:"uuid" example:"56320ee9-6af6-11ed-a7ba-f220afe5e4a9"`
	Position *int       `json:"position" binding:"omitempty,min=0" example:"0"`
}

type ReorderMenuRequest struct {
	Position *int `json:"position" binding:"required,min=0" example:"0"`
}
