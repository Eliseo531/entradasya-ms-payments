package service

import (
	"context"
	"testing"

	"entradasya/ms-payments/internal/model"
	"entradasya/ms-payments/internal/repository"
)

func TestPaymentService_ProcessPayment(t *testing.T) {
	ctx := context.Background()

	t.Run("should approve payment for standard card", func(t *testing.T) {
		repo := repository.NewMockPaymentRepository()
		svc := NewPaymentService(repo)

		req := &model.PaymentRequest{
			OrderID:    "order-uuid-1",
			Amount:     150.00,
			Currency:   "BOB",
			CardNumber: "4532000012345678",
			CardHolder: "JUAN PEREZ",
			ExpiryDate: "12/28",
			CVV:        "123",
		}

		resp, code, err := svc.ProcessPayment(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != 200 {
			t.Errorf("expected status code 200, got %d", code)
		}
		if resp.Status != string(model.StatusApproved) {
			t.Errorf("expected status APPROVED, got %s", resp.Status)
		}
		if resp.AuthCode == nil || *resp.AuthCode == "" {
			t.Errorf("expected non-empty auth_code")
		}
		if resp.Amount == nil || *resp.Amount != 150.00 {
			t.Errorf("expected amount 150.00, got %v", resp.Amount)
		}
		if resp.Currency == nil || *resp.Currency != "BOB" {
			t.Errorf("expected currency BOB, got %v", resp.Currency)
		}
	})

	t.Run("should reject payment with FONDOS_INSUFICIENTES for card ending in 0000", func(t *testing.T) {
		repo := repository.NewMockPaymentRepository()
		svc := NewPaymentService(repo)

		req := &model.PaymentRequest{
			OrderID:    "order-uuid-2",
			Amount:     250.00,
			Currency:   "BOB",
			CardNumber: "4000123456780000",
		}

		resp, code, err := svc.ProcessPayment(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != 402 {
			t.Errorf("expected status code 402, got %d", code)
		}
		if resp.Status != string(model.StatusRejected) {
			t.Errorf("expected status REJECTED, got %s", resp.Status)
		}
		if resp.RejectionReason == nil || *resp.RejectionReason != string(model.ReasonFondosInsuficientes) {
			t.Errorf("expected rejection reason FONDOS_INSUFICIENTES, got %v", resp.RejectionReason)
		}
	})

	t.Run("should reject payment with TARJETA_EXPIRADA_O_BLOQUEADA for card ending in 9999", func(t *testing.T) {
		repo := repository.NewMockPaymentRepository()
		svc := NewPaymentService(repo)

		req := &model.PaymentRequest{
			OrderID:    "order-uuid-3",
			Amount:     300.00,
			Currency:   "BOB",
			CardNumber: "4000123456789999",
		}

		resp, code, err := svc.ProcessPayment(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != 402 {
			t.Errorf("expected status code 402, got %d", code)
		}
		if resp.Status != string(model.StatusRejected) {
			t.Errorf("expected status REJECTED, got %s", resp.Status)
		}
		if resp.RejectionReason == nil || *resp.RejectionReason != string(model.ReasonTarjetaExpiradaOBloqueada) {
			t.Errorf("expected rejection reason TARJETA_EXPIRADA_O_BLOQUEADA, got %v", resp.RejectionReason)
		}
	})

	t.Run("should reject payment with SOSPECHA_DE_FRAUDE for card ending in 1111", func(t *testing.T) {
		repo := repository.NewMockPaymentRepository()
		svc := NewPaymentService(repo)

		req := &model.PaymentRequest{
			OrderID:    "order-uuid-4",
			Amount:     500.00,
			Currency:   "BOB",
			CardNumber: "4000123456781111",
		}

		resp, code, err := svc.ProcessPayment(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != 402 {
			t.Errorf("expected status code 402, got %d", code)
		}
		if resp.Status != string(model.StatusRejected) {
			t.Errorf("expected status REJECTED, got %s", resp.Status)
		}
		if resp.RejectionReason == nil || *resp.RejectionReason != string(model.ReasonSospechaDeFraude) {
			t.Errorf("expected rejection reason SOSPECHA_DE_FRAUDE, got %v", resp.RejectionReason)
		}
	})

	t.Run("should reject request when amount is <= 0", func(t *testing.T) {
		repo := repository.NewMockPaymentRepository()
		svc := NewPaymentService(repo)

		reqZero := &model.PaymentRequest{
			OrderID:    "order-uuid-5",
			Amount:     0,
			CardNumber: "4532000012345678",
		}
		_, codeZero, errZero := svc.ProcessPayment(ctx, reqZero)
		if errZero != ErrInvalidAmount || codeZero != 400 {
			t.Errorf("expected ErrInvalidAmount and 400 for amount 0, got err: %v, code: %d", errZero, codeZero)
		}

		reqNeg := &model.PaymentRequest{
			OrderID:    "order-uuid-6",
			Amount:     -50.00,
			CardNumber: "4532000012345678",
		}
		_, codeNeg, errNeg := svc.ProcessPayment(ctx, reqNeg)
		if errNeg != ErrInvalidAmount || codeNeg != 400 {
			t.Errorf("expected ErrInvalidAmount and 400 for negative amount, got err: %v, code: %d", errNeg, codeNeg)
		}
	})

	t.Run("should guarantee idempotency on duplicate order_id", func(t *testing.T) {
		repo := repository.NewMockPaymentRepository()
		svc := NewPaymentService(repo)

		orderID := "order-idempotent-test"
		req := &model.PaymentRequest{
			OrderID:    orderID,
			Amount:     200.00,
			Currency:   "BOB",
			CardNumber: "4532000012345678",
		}

		// Primera llamada
		resp1, code1, err1 := svc.ProcessPayment(ctx, req)
		if err1 != nil || code1 != 200 {
			t.Fatalf("first payment failed: %v", err1)
		}

		// Segunda llamada con mismo order_id pero diferentes otros datos (ej. reintento)
		reqRetry := &model.PaymentRequest{
			OrderID:    orderID,
			Amount:     200.00,
			Currency:   "BOB",
			CardNumber: "4532000012340000", // tarjeta terminada en 0000, pero debe retornar el original aprobado
		}

		resp2, code2, err2 := svc.ProcessPayment(ctx, reqRetry)
		if err2 != nil {
			t.Fatalf("second call error: %v", err2)
		}
		if code2 != 200 {
			t.Errorf("expected status code 200 for idempotent hit, got %d", code2)
		}
		if resp2.TransactionID != resp1.TransactionID {
			t.Errorf("expected transaction_id %s, got %s", resp1.TransactionID, resp2.TransactionID)
		}
		if resp2.Status != string(model.StatusApproved) {
			t.Errorf("expected status APPROVED from previous record, got %s", resp2.Status)
		}
		if resp2.AuthCode == nil || *resp2.AuthCode != *resp1.AuthCode {
			t.Errorf("expected identical auth_code %v, got %v", resp1.AuthCode, resp2.AuthCode)
		}
	})

	t.Run("should query transaction detail by order_id", func(t *testing.T) {
		repo := repository.NewMockPaymentRepository()
		svc := NewPaymentService(repo)

		orderID := "order-detail-test"
		req := &model.PaymentRequest{
			OrderID:    orderID,
			Amount:     180.00,
			Currency:   "BOB",
			CardNumber: "4532000012345678",
		}

		resp, _, _ := svc.ProcessPayment(ctx, req)

		detail, err := svc.GetPaymentByOrderID(ctx, orderID)
		if err != nil {
			t.Fatalf("unexpected error getting payment detail: %v", err)
		}
		if detail.TransactionID != resp.TransactionID {
			t.Errorf("expected transaction_id %s, got %s", resp.TransactionID, detail.TransactionID)
		}
		if detail.CardLastFour != "5678" {
			t.Errorf("expected last four 5678, got %s", detail.CardLastFour)
		}
		if detail.Amount != 180.00 {
			t.Errorf("expected amount 180.00, got %f", detail.Amount)
		}
	})
}
