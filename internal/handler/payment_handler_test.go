package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"entradasya/ms-payments/internal/middleware"
	"entradasya/ms-payments/internal/model"
	"entradasya/ms-payments/internal/repository"
	"entradasya/ms-payments/internal/service"

	"github.com/gin-gonic/gin"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(_ context.Context) error {
	return m.err
}

const testInternalToken = "internal-service-secret-entradasya-2026"

func setupTestApp(pingerErr error) (*gin.Engine, *repository.MockPaymentRepository) {
	gin.SetMode(gin.TestMode)

	repo := repository.NewMockPaymentRepository()
	paymentSvc := service.NewPaymentService(repo)
	paymentHdl := NewPaymentHandler(paymentSvc)

	pinger := &mockPinger{err: pingerErr}
	healthHdl := NewHealthHandler(pinger)

	r := gin.New()
	r.Use(gin.Recovery())

	// Health routes
	r.GET("/health/live", healthHdl.Live)
	r.GET("/health/ready", healthHdl.Ready)

	// API routes
	api := r.Group("/api/v1")
	api.Use(middleware.InternalAuth(testInternalToken))
	{
		api.POST("/payments", paymentHdl.ProcessPayment)
		api.GET("/payments/order/:orderId", paymentHdl.GetPaymentByOrderID)
	}

	return r, repo
}

func TestHealthEndpoints(t *testing.T) {
	t.Run("GET /health/live returns 200 UP", func(t *testing.T) {
		r, _ := setupTestApp(nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/health/live", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var body map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["status"] != "UP" {
			t.Errorf("expected status UP, got %s", body["status"])
		}
	})

	t.Run("GET /health/ready returns 200 when database ping succeeds", func(t *testing.T) {
		r, _ := setupTestApp(nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var body map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["status"] != "READY" {
			t.Errorf("expected status READY, got %s", body["status"])
		}
	})

	t.Run("GET /health/ready returns 503 when database ping fails", func(t *testing.T) {
		r, _ := setupTestApp(errors.New("connection refused"))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status 503, got %d", w.Code)
		}

		var body map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["status"] != "DOWN" {
			t.Errorf("expected status DOWN, got %s", body["status"])
		}
	})
}

func TestPaymentHandler_Security(t *testing.T) {
	r, _ := setupTestApp(nil)

	t.Run("should reject request with 403 when X-Internal-Token is missing", func(t *testing.T) {
		payload := map[string]interface{}{
			"order_id":    "ord-1",
			"amount":      100.0,
			"card_number": "4532000012345678",
		}
		data, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", w.Code)
		}
	})

	t.Run("should reject request with 403 when X-Internal-Token is incorrect", func(t *testing.T) {
		payload := map[string]interface{}{
			"order_id":    "ord-1",
			"amount":      100.0,
			"card_number": "4532000012345678",
		}
		data, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Token", "invalid-secret")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", w.Code)
		}
	})
}

func TestPaymentHandler_ProcessPayment(t *testing.T) {
	r, _ := setupTestApp(nil)

	t.Run("should approve payment with 200 OK for valid card", func(t *testing.T) {
		payload := model.PaymentRequest{
			OrderID:    "9a8b7c6d-5e4f-3a2b-1c0d-9e8f7a6b5c4d",
			Amount:     400.00,
			Currency:   "BOB",
			CardNumber: "4532000012345678",
			CardHolder: "JUAN PEREZ",
			ExpiryDate: "12/28",
			CVV:        "123",
		}
		data, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Token", testInternalToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp model.PaymentResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Status != "APPROVED" {
			t.Errorf("expected APPROVED, got %s", resp.Status)
		}
		if resp.AuthCode == nil || *resp.AuthCode == "" {
			t.Errorf("expected auth_code to be present")
		}
		if resp.OrderID != payload.OrderID {
			t.Errorf("expected order_id %s, got %s", payload.OrderID, resp.OrderID)
		}
	})

	t.Run("should reject payment with 402 for card ending in 0000", func(t *testing.T) {
		payload := model.PaymentRequest{
			OrderID:    "9a8b7c6d-5e4f-3a2b-1c0d-000000000000",
			Amount:     400.00,
			Currency:   "BOB",
			CardNumber: "4532000012340000",
		}
		data, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Token", testInternalToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusPaymentRequired {
			t.Fatalf("expected 402 Payment Required, got %d", w.Code)
		}

		var resp model.PaymentResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.Status != "REJECTED" {
			t.Errorf("expected REJECTED, got %s", resp.Status)
		}
		if resp.RejectionReason == nil || *resp.RejectionReason != "FONDOS_INSUFICIENTES" {
			t.Errorf("expected rejection_reason FONDOS_INSUFICIENTES, got %v", resp.RejectionReason)
		}
	})

	t.Run("should reject payment with 402 for card ending in 9999", func(t *testing.T) {
		payload := model.PaymentRequest{
			OrderID:    "9a8b7c6d-5e4f-3a2b-1c0d-999999999999",
			Amount:     400.00,
			Currency:   "BOB",
			CardNumber: "4532000012349999",
		}
		data, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Token", testInternalToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusPaymentRequired {
			t.Fatalf("expected 402 Payment Required, got %d", w.Code)
		}

		var resp model.PaymentResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.Status != "REJECTED" {
			t.Errorf("expected REJECTED, got %s", resp.Status)
		}
		if resp.RejectionReason == nil || *resp.RejectionReason != "TARJETA_EXPIRADA_O_BLOQUEADA" {
			t.Errorf("expected rejection_reason TARJETA_EXPIRADA_O_BLOQUEADA, got %v", resp.RejectionReason)
		}
	})

	t.Run("should reject payment with 402 for card ending in 1111", func(t *testing.T) {
		payload := model.PaymentRequest{
			OrderID:    "9a8b7c6d-5e4f-3a2b-1c0d-111111111111",
			Amount:     400.00,
			Currency:   "BOB",
			CardNumber: "4532000012341111",
		}
		data, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Token", testInternalToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusPaymentRequired {
			t.Fatalf("expected 402 Payment Required, got %d", w.Code)
		}

		var resp model.PaymentResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)

		if resp.Status != "REJECTED" {
			t.Errorf("expected REJECTED, got %s", resp.Status)
		}
		if resp.RejectionReason == nil || *resp.RejectionReason != "SOSPECHA_DE_FRAUDE" {
			t.Errorf("expected rejection_reason SOSPECHA_DE_FRAUDE, got %v", resp.RejectionReason)
		}
	})

	t.Run("should reject payment with 400 when amount <= 0", func(t *testing.T) {
		payload := model.PaymentRequest{
			OrderID:    "ord-zero",
			Amount:     0,
			CardNumber: "4532000012345678",
		}
		data, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Token", testInternalToken)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("should return original payment idempotently on duplicate call", func(t *testing.T) {
		orderID := "idempotent-order-123"
		payload := model.PaymentRequest{
			OrderID:    orderID,
			Amount:     120.00,
			Currency:   "BOB",
			CardNumber: "4532000012345678",
		}
		data, _ := json.Marshal(payload)

		// Primera llamada
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("X-Internal-Token", testInternalToken)
		r.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Fatalf("first call expected 200, got %d", w1.Code)
		}

		var resp1 model.PaymentResponse
		_ = json.Unmarshal(w1.Body.Bytes(), &resp1)

		// Segunda llamada con el mismo order_id
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("X-Internal-Token", testInternalToken)
		r.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Fatalf("second call expected 200, got %d", w2.Code)
		}

		var resp2 model.PaymentResponse
		_ = json.Unmarshal(w2.Body.Bytes(), &resp2)

		if resp2.TransactionID != resp1.TransactionID {
			t.Errorf("expected idempotent transaction_id %s, got %s", resp1.TransactionID, resp2.TransactionID)
		}
		if *resp2.AuthCode != *resp1.AuthCode {
			t.Errorf("expected identical auth_code %s, got %s", *resp1.AuthCode, *resp2.AuthCode)
		}
	})
}

func TestPaymentHandler_GetPaymentByOrderID(t *testing.T) {
	r, _ := setupTestApp(nil)

	orderID := "order-lookup-test"

	// Crear pago previo
	payload := model.PaymentRequest{
		OrderID:    orderID,
		Amount:     350.00,
		Currency:   "BOB",
		CardNumber: "4532000012345678",
	}
	data, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", testInternalToken)
	r.ServeHTTP(w, req)

	t.Run("should retrieve payment by orderId with 200 OK", func(t *testing.T) {
		wGet := httptest.NewRecorder()
		reqGet, _ := http.NewRequest(http.MethodGet, "/api/v1/payments/order/"+orderID, nil)
		reqGet.Header.Set("X-Internal-Token", testInternalToken)
		r.ServeHTTP(wGet, reqGet)

		if wGet.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", wGet.Code)
		}

		var detail model.TransactionDetailResponse
		if err := json.Unmarshal(wGet.Body.Bytes(), &detail); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if detail.OrderID != orderID {
			t.Errorf("expected order_id %s, got %s", orderID, detail.OrderID)
		}
		if detail.Status != "APPROVED" {
			t.Errorf("expected status APPROVED, got %s", detail.Status)
		}
		if detail.Amount != 350.00 {
			t.Errorf("expected amount 350.00, got %f", detail.Amount)
		}
	})

	t.Run("should return 404 Not Found for non-existing orderId", func(t *testing.T) {
		wGet := httptest.NewRecorder()
		reqGet, _ := http.NewRequest(http.MethodGet, "/api/v1/payments/order/non-existent-order", nil)
		reqGet.Header.Set("X-Internal-Token", testInternalToken)
		r.ServeHTTP(wGet, reqGet)

		if wGet.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", wGet.Code)
		}
	})
}
