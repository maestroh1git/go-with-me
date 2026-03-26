package lib

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// Params holds parsed pagination parameters from an HTTP request.
type Params struct {
	Page     int
	PageSize int
}

// Offset returns the SQL OFFSET for this page.
func (p Params) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// Meta holds pagination metadata for API responses.
type Meta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

// ParseParams reads page and page_size query parameters from the Gin context.
// Defaults: page=1, page_size=20. page_size is capped at 100.
func ParseParams(c *gin.Context) Params {
	page, _ := strconv.Atoi(c.DefaultQuery("page", strconv.Itoa(defaultPage)))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultPageSize)))

	if page < 1 {
		page = defaultPage
	}
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}

	return Params{Page: page, PageSize: pageSize}
}

// NewMeta builds a Meta response from params and a total count.
func NewMeta(params Params, total int64) Meta {
	totalPages := total / int64(params.PageSize)
	if total%int64(params.PageSize) > 0 {
		totalPages++
	}
	return Meta{
		Page:       params.Page,
		PageSize:   params.PageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}
