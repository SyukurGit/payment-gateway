package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"paymentg/internal/database"
	"paymentg/internal/model"
	"paymentg/internal/service"
)

type HealthHandler struct {
	db     *database.DB
	poller *service.Poller
}

func NewHealthHandler(db *database.DB, poller *service.Poller) *HealthHandler {
	return &HealthHandler{
		db:     db,
		poller: poller,
	}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	pendingOrders, _ := h.db.CountPendingOrders()
	
	tokenValid, tokenError, lastPollAt, lastPollOK := h.poller.Status()
	
	updatedAtStr, _ := h.db.GetConfig("shopee_token_updated_at")
	var tokenAgeHours float64
	if updatedAtStr != "" {
		if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
			tokenAgeHours = time.Since(updatedAt).Hours()
		}
	}

	status := "degraded"
	msg := "Update token via PUT /api/config/token"
	if tokenValid {
		status = "ok"
		msg = ""
	}

	lastPollAtStr := ""
	if !lastPollAt.IsZero() {
		lastPollAtStr = lastPollAt.Format(time.RFC3339)
	}

	resp := model.HealthResponse{
		Status:          status,
		TokenValid:      tokenValid,
		TokenUpdatedAt:  updatedAtStr,
		TokenAgeHours:   tokenAgeHours,
		TokenError:      tokenError,
		PendingOrders:   pendingOrders,
		LastPollAt:      lastPollAtStr,
		LastPollSuccess: lastPollOK,
		Message:         msg,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}
