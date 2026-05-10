package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
)

// CreateRole godoc
// @Router       /role [POST]
// @Security     ApiKeyAuth
// @Summary      Create role
// @Tags         role
// @Accept       json
// @Produce      json
// @Param        request body models.CreateRole true "role"
// @Success      201  {object}  map[string]string
// @Failure      400  {object}  models.Response
// @Failure      500  {object}  models.Response
func (h Handler) CreateRole(c *gin.Context) {
	var req models.CreateRole
	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request body", http.StatusBadRequest, err.Error())
		return
	}

	createdBy := c.GetString("user_id") // JWT middleware orqali user_id olinadi deb faraz qilamiz

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := h.services.Role().Create(ctx, req.Name, createdBy)
	if err != nil {
		handleResponse(c, h.log, "failed to create role", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "role created", http.StatusCreated, gin.H{"id": id})
}

// UpdateRole godoc
// @Router       /role/{id} [PUT]
// @Security     ApiKeyAuth
// @Summary      Update role
// @Tags         role
// @Accept       json
// @Produce      json
// @Param        id path string true "role_id"
// @Param        request body models.UpdateRole true "update info"
// @Success      200  {object}  models.Response
// @Failure      400  {object}  models.Response
// @Failure      500  {object}  models.Response
func (h Handler) UpdateRole(c *gin.Context) {
	var req models.UpdateRole
	req.ID = c.Param("id")

	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request body", http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.services.Role().Update(ctx, req.ID, req.Name); err != nil {
		handleResponse(c, h.log, "failed to update role", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "role updated", http.StatusOK, nil)
}

// ListRoles godoc
// @Router       /roles [GET]
// @Security     ApiKeyAuth
// @Summary      List all roles
// @Tags         role
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.RoleListResponse
// @Failure      500  {object}  models.Response
func (h Handler) ListRoles(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	roles, err := h.services.Role().GetAll(ctx)
	if err != nil {
		handleResponse(c, h.log, "failed to fetch roles", http.StatusInternalServerError, err.Error())
		return
	}

	resp := models.RoleListResponse{
		Roles: roles,
	}

	handleResponse(c, h.log, "roles fetched", http.StatusOK, resp)
}
