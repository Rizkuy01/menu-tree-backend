package model

import (
	"time"

	"github.com/google/uuid"
)

// Menu mewakili satu baris di tabel menus.
type Menu struct {
	ID         uuid.UUID  `json:"id" swaggertype:"string" format:"uuid" example:"968d9ed2-8e06-4988-bd2a-b94255f4bbab"`
	Name       string     `json:"name" example:"System Code"`
	ParentID   *uuid.UUID `json:"parent_id" swaggertype:"string" format:"uuid" example:"56320ee9-6af6-11ed-a7ba-f220afe5e4a9"`
	ParentName *string    `json:"parent_name,omitempty" example:"Systems"`
	Depth      int        `json:"depth" example:"2"`
	Position   int        `json:"position" example:"0"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// MenuNode adalah Menu dalam bentuk tree (punya children).
type MenuNode struct {
	ID       uuid.UUID   `json:"id" swaggertype:"string" format:"uuid" example:"968d9ed2-8e06-4988-bd2a-b94255f4bbab"`
	Name     string      `json:"name" example:"System Management"`
	ParentID *uuid.UUID  `json:"parent_id" swaggertype:"string" format:"uuid"`
	Depth    int         `json:"depth" example:"0"`
	Position int         `json:"position" example:"0"`
	Children []*MenuNode `json:"children"`
}
