package handler

import (
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"paymentg/internal/config"
	"paymentg/internal/database"
	"paymentg/internal/model"
	"paymentg/internal/util"
)

type AdminHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAdminHandler(db *database.DB, cfg *config.Config) *AdminHandler {
	return &AdminHandler{
		db:  db,
		cfg: cfg,
	}
}

func (h *AdminHandler) UpdateToken(c *gin.Context) {
	var req model.UpdateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if err := h.db.SetConfig("shopee_token", req.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to save token"})
		return
	}

	h.db.SetConfig("shopee_token_updated_at", time.Now().Format(time.RFC3339))
	c.JSON(http.StatusOK, gin.H{"success": true, "data": "token updated successfully"})
}

func (h *AdminHandler) CreateApp(c *gin.Context) {
	var req model.CreateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	app := model.App{
		ID:            util.NewID(),
		Name:          req.Name,
		APIKey:        util.NewAPIKey(),
		WebhookURL:    req.WebhookURL,
		WebhookSecret: util.NewSecret(),
		IsActive:      true,
		CreatedAt:     time.Now().Format(time.RFC3339),
	}

	if err := h.db.CreateApp(&app); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create app"})
		return
	}

	resp := model.CreateAppResponse{
		ID:            app.ID,
		Name:          app.Name,
		APIKey:        app.APIKey,
		WebhookURL:    app.WebhookURL,
		WebhookSecret: app.WebhookSecret,
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": resp})
}

func (h *AdminHandler) ListApps(c *gin.Context) {
	apps, err := h.db.ListApps()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to fetch apps"})
		return
	}

	// Mask API Key
	for i := range apps {
		if len(apps[i].APIKey) > 8 {
			apps[i].APIKey = apps[i].APIKey[:8] + "..."
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": apps})
}

func (h *AdminHandler) DeleteApp(c *gin.Context) {
	id := c.Param("id")

	if err := h.db.DeleteApp(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to delete app"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": "app deleted"})
}

func (h *AdminHandler) ListOrders(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	orders, err := h.db.ListAllOrders(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to fetch orders"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": orders})
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	stats, err := h.db.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to fetch stats"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func (h *AdminHandler) TriggerTokenRefresh(c *gin.Context) {
	if _, err := os.Stat("refresh_token.py"); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "refresh_token.py not found"})
		return
	}

	cmd := exec.Command("python", "refresh_token.py")
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error(), "output": string(out)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": string(out)})
}

type PINRequest struct {
	PIN string `json:"pin" binding:"required"`
}

func (h *AdminHandler) VerifyPIN(c *gin.Context) {
	var req PINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "PIN wajib diisi"})
		return
	}

	if !h.db.VerifyPIN(req.PIN) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "PIN akses salah"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"message":   "PIN valid",
			"admin_key": h.cfg.AdminKey,
		},
	})
}
