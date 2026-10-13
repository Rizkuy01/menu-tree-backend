package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"menu-tree-backend/internal/model"
)

type ErrorDetail struct {
	Code    string `json:"code" example:"VALIDATION_ERROR"`
	Message string `json:"message" example:"name must not be empty"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{Error: ErrorDetail{Code: code, Message: message}})
}

// handleServiceError menerjemahkan error dari service ke status HTTP.
func handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, model.ErrParentNotFound):
		respondError(c, http.StatusNotFound, "PARENT_NOT_FOUND", err.Error())
	case errors.Is(err, model.ErrInvalidName):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	default:
		log.Printf("internal error: %v", err)
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
