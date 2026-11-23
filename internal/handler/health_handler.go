package handler

import (
	"context"
	"net/http"
	"time"

	"entradasya/ms-payments/internal/database"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db database.Pinger
}

func NewHealthHandler(db database.Pinger) *HealthHandler {
	return &HealthHandler{db: db}
}

// Live godoc
// @Summary Liveness probe
// @Description Devuelve 200 si el proceso está vivo
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health/live [get]
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "UP",
		"service":   "ms-payments",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Ready godoc
// @Summary Readiness probe
// @Description Valida si la base de datos está accesible
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /health/ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "DOWN",
			"error":   "database connection is not configured",
			"service": "ms-payments",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "DOWN",
			"error":   "database unreachable: " + err.Error(),
			"service": "ms-payments",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "READY",
		"service":   "ms-payments",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
