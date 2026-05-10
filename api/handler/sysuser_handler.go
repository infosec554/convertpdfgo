package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/security"
)

// CreateSysUser godoc
// @Router       /sysuser [POST]
// @Security     ApiKeyAuth
// @Summary      Create system user
// @Description  Superadmin tomonidan tizim user (admin, buxgalter) yaratish
// @Tags         sysuser
// @Accept       json
// @Produce      json
// @Param        request body models.CreateSysUser true "sysuser info"
// @Success      201  {object}  map[string]string
// @Failure      400  {object}  models.Response
// @Failure      500  {object}  models.Response
func (h Handler) CreateSysUser(c *gin.Context) {
	var req models.CreateSysUser

	if err := c.ShouldBindJSON(&req); err != nil {
		handleResponse(c, h.log, "invalid request body", http.StatusBadRequest, err.Error())
		return
	}

	// Token dan user_id olish (middleware orqali qo‘yilgan deb hisoblaymiz)
	createdBy := c.GetString("user_id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Avval sysuser mavjudligini tekshiramiz
	_, _, status, err := h.services.SysUser().GetByPhone(ctx, req.Phone)
	if err == nil && status == "active" {
		handleResponse(c, h.log, "sysuser already exists", http.StatusBadRequest, "duplicate phone number")
		return
	}

	// Role ID larni mavjudligini tekshirish
	for _, roleID := range req.Roles {
		exist, err := h.services.Role().Exists(ctx, roleID)
		if err != nil {
			handleResponse(c, h.log, "error checking role", http.StatusInternalServerError, err.Error())
			return
		}
		if !exist {
			handleResponse(c, h.log, "role not found", http.StatusBadRequest, "role id not found: "+roleID)
			return
		}
	}

	// Parolni hash qilish
	hashedPassword, err := security.HashPassword(req.Password)
	if err != nil {
		handleResponse(c, h.log, "failed to hash password", http.StatusInternalServerError, err.Error())
		return
	}

	// Sysuser yaratish
	sysuserID, err := h.services.SysUser().Create(ctx, req.Name, req.Phone, hashedPassword, createdBy)
	if err != nil {
		handleResponse(c, h.log, "failed to create sysuser", http.StatusInternalServerError, err.Error())
		return
	}

	// Rollarni biriktirish
	err = h.services.SysUser().AssignRoles(ctx, sysuserID, req.Roles)
	if err != nil {
		handleResponse(c, h.log, "failed to assign roles", http.StatusInternalServerError, err.Error())
		return
	}

	handleResponse(c, h.log, "sysuser created", http.StatusCreated, gin.H{"id": sysuserID})
}
