package lib

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// envelope is the standard JSON response wrapper for all API responses.
type envelope struct {
	Success bool        `json:"success"`
	Data    any         `json:"data,omitempty"`
	Error   *errorBody  `json:"error,omitempty"`
	Meta    any         `json:"meta,omitempty"`
}

type errorBody struct {
	Code    ErrorCode         `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Success writes a 200 JSON response with data.
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Success: true, Data: data})
}

// SuccessWithMeta writes a 200 JSON response with data and pagination/meta.
func SuccessWithMeta(c *gin.Context, data any, meta any) {
	c.JSON(http.StatusOK, envelope{Success: true, Data: data, Meta: meta})
}

// Created writes a 201 JSON response with data.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, envelope{Success: true, Data: data})
}

// NoContent writes a 204 response with no body.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Fail writes an error JSON response. Accepts *APIError or a generic error.
func Fail(c *gin.Context, err error) {
	apiErr, ok := err.(*APIError)
	if !ok {
		apiErr = ErrInternalServer
	}
	c.JSON(apiErr.Status, envelope{
		Success: false,
		Error: &errorBody{
			Code:    apiErr.Code,
			Message: apiErr.Message,
			Details: apiErr.Details,
		},
	})
}

// FailStatus writes an error JSON response with an explicit HTTP status code.
func FailStatus(c *gin.Context, status int, err *APIError) {
	c.JSON(status, envelope{
		Success: false,
		Error: &errorBody{
			Code:    err.Code,
			Message: err.Message,
			Details: err.Details,
		},
	})
}
