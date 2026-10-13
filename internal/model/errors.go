package model

import "errors"

var (
	ErrNotFound        = errors.New("menu not found")
	ErrParentNotFound  = errors.New("parent menu not found")
	ErrInvalidName     = errors.New("name must not be empty")
	ErrCycle           = errors.New("cannot move a menu into itself or its own descendant")
	ErrInvalidPosition = errors.New("position must be zero or greater")
)
