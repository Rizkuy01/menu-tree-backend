package model

import "errors"

var (
	ErrNotFound       = errors.New("menu not found")
	ErrParentNotFound = errors.New("parent menu not found")
	ErrInvalidName    = errors.New("name must not be empty")
)
