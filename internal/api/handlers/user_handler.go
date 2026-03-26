package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/go-with-me/app/internal/api/middleware"
	"github.com/go-with-me/app/internal/lib"
	"github.com/go-with-me/app/internal/models"
	"github.com/go-with-me/app/internal/repository/interfaces"
	"github.com/go-with-me/app/internal/services"
)

// UserHandler handles HTTP requests for the users domain.
type UserHandler struct {
	users *services.UserService
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(users *services.UserService) *UserHandler {
	return &UserHandler{users: users}
}

// List returns a paginated list of users.
// GET /api/v1/users
func (h *UserHandler) List(c *gin.Context) {
	params := lib.ParseParams(c)

	var role *models.UserRole
	if r := c.Query("role"); r != "" {
		ur := models.UserRole(r)
		role = &ur
	}

	var search *string
	if s := c.Query("search"); s != "" {
		search = &s
	}

	users, total, err := h.users.List(c.Request.Context(), interfaces.ListUsersParams{
		Role:   role,
		Search: search,
		Params: params,
	})
	if err != nil {
		lib.Fail(c, err)
		return
	}

	lib.SuccessWithMeta(c, mapUsers(users), lib.NewMeta(params, total))
}

// Get returns a single user by ID.
// GET /api/v1/users/:id
func (h *UserHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		lib.Fail(c, lib.BadRequest("invalid user ID"))
		return
	}

	u, err := h.users.Get(c.Request.Context(), id)
	if err != nil {
		lib.Fail(c, err)
		return
	}

	lib.Success(c, mapUser(u))
}

// updateUserRequest is the body for PATCH /api/v1/users/:id.
type updateUserRequest struct {
	Name *string          `json:"name"`
	Role *models.UserRole `json:"role"`
}

// Update applies a partial update to a user.
// PATCH /api/v1/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		lib.Fail(c, lib.BadRequest("invalid user ID"))
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.Fail(c, lib.BadRequest(err.Error()))
		return
	}

	u, err := h.users.Update(c.Request.Context(), id, interfaces.UpdateUserParams{
		Name: req.Name,
		Role: req.Role,
	})
	if err != nil {
		lib.Fail(c, err)
		return
	}

	lib.Success(c, mapUser(u))
}

// inviteRequest is the body for POST /api/v1/users/invite.
type inviteRequest struct {
	Email string           `json:"email" binding:"required,email"`
	Name  string           `json:"name" binding:"required"`
	Role  models.UserRole  `json:"role" binding:"required"`
}

// Invite creates an invite token for a new user.
// POST /api/v1/users/invite
func (h *UserHandler) Invite(c *gin.Context) {
	var req inviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		lib.Fail(c, lib.BadRequest(err.Error()))
		return
	}

	invitedByID := middleware.GetUserID(c)

	token, err := h.users.Invite(c.Request.Context(), req.Email, req.Name, req.Role, invitedByID)
	if err != nil {
		lib.Fail(c, err)
		return
	}

	lib.Created(c, gin.H{
		"invite_token": token,
		"message":      "send this token to the user — it expires after the configured TTL",
	})
}

// Deactivate marks a user as inactive.
// DELETE /api/v1/users/:id
func (h *UserHandler) Deactivate(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		lib.Fail(c, lib.BadRequest("invalid user ID"))
		return
	}

	if err := h.users.Deactivate(c.Request.Context(), id); err != nil {
		lib.Fail(c, err)
		return
	}

	lib.NoContent(c)
}

// userResponse is the JSON-safe view of a user (no password hash).
type userResponse struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	TOTPEnabled  bool   `json:"totp_enabled"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func mapUser(u *models.User) userResponse {
	return userResponse{
		ID:          u.ID.String(),
		Email:       u.Email,
		Name:        u.Name,
		Role:        string(u.Role),
		TOTPEnabled: u.TOTPEnabled,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func mapUsers(users []*models.User) []userResponse {
	out := make([]userResponse, 0, len(users))
	for _, u := range users {
		out = append(out, mapUser(u))
	}
	return out
}
