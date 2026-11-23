package handler

import (
	"errors"
	"net/http"

	"entradasya/ms-payments/internal/model"
	"entradasya/ms-payments/internal/repository"
	"entradasya/ms-payments/internal/service"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service service.PaymentService
}

func NewPaymentHandler(service service.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

// ProcessPayment godoc
// @Summary Procesa un pago de orden de forma determinista e idempotente
// @Tags Payments
// @Accept json
// @Produce json
// @Param X-Internal-Token header string true "Token interno de servicio"
// @Param body body model.PaymentRequest true "Datos de pago"
// @Success 200 {object} model.PaymentResponse
// @Failure 400 {object} map[string]string
// @Failure 402 {object} model.PaymentResponse
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/payments [post]
func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	var req model.PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "Invalid request body: " + err.Error(),
		})
		return
	}

	resp, statusCode, err := h.service.ProcessPayment(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAmount) || errors.Is(err, service.ErrInvalidOrder) || errors.Is(err, service.ErrInvalidCard) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Bad Request",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(statusCode, resp)
}

// GetPaymentByOrderID godoc
// @Summary Consulta la transacción asociada a un order_id
// @Tags Payments
// @Produce json
// @Param X-Internal-Token header string true "Token interno de servicio"
// @Param orderId path string true "ID de la orden"
// @Success 200 {object} model.TransactionDetailResponse
// @Failure 404 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/payments/order/{orderId} [get]
func (h *PaymentHandler) GetPaymentByOrderID(c *gin.Context) {
	orderID := c.Param("orderId")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": "orderId parameter is required",
		})
		return
	}

	detail, err := h.service.GetPaymentByOrderID(c.Request.Context(), orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Not Found",
				"message": "Transaction not found for the specified order_id",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal Server Error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, detail)
}
