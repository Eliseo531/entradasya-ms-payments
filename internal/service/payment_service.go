package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"entradasya/ms-payments/internal/model"
	"entradasya/ms-payments/internal/repository"

	"github.com/google/uuid"
)

var (
	ErrInvalidAmount = errors.New("amount must be greater than 0")
	ErrInvalidOrder  = errors.New("order_id is required")
	ErrInvalidCard   = errors.New("card_number is required")
)

type PaymentService interface {
	ProcessPayment(ctx context.Context, req *model.PaymentRequest) (*model.PaymentResponse, int, error)
	GetPaymentByOrderID(ctx context.Context, orderID string) (*model.TransactionDetailResponse, error)
}

type paymentService struct {
	repo repository.PaymentRepository
}

func NewPaymentService(repo repository.PaymentRepository) PaymentService {
	return &paymentService{repo: repo}
}

func (s *paymentService) ProcessPayment(ctx context.Context, req *model.PaymentRequest) (*model.PaymentResponse, int, error) {
	if req.OrderID == "" {
		return nil, 400, ErrInvalidOrder
	}
	if req.Amount <= 0 {
		return nil, 400, ErrInvalidAmount
	}

	cleanCard := strings.ReplaceAll(strings.ReplaceAll(req.CardNumber, " ", ""), "-", "")
	if cleanCard == "" {
		return nil, 400, ErrInvalidCard
	}

	// 1. Verificación de Idempotencia por order_id
	existingTx, err := s.repo.GetByOrderID(ctx, req.OrderID)
	if err == nil && existingTx != nil {
		log.Printf("[IDEMPOTENT_HIT] Order ID %s already processed. Returning original transaction %s", req.OrderID, existingTx.ID)
		resp, statusCode := buildResponseFromTransaction(existingTx)
		return resp, statusCode, nil
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, 500, fmt.Errorf("error checking existing transaction: %w", err)
	}

	// 2. Extraer últimos 4 dígitos
	var lastFour string
	if len(cleanCard) >= 4 {
		lastFour = cleanCard[len(cleanCard)-4:]
	} else {
		lastFour = fmt.Sprintf("%04s", cleanCard)
	}

	currency := req.Currency
	if currency == "" {
		currency = "BOB"
	}

	// 3. Reglas deterministas de cobro
	var (
		status          model.TransactionStatus
		authCode        *string
		rejectionReason *string
		httpStatus      int
	)

	switch lastFour {
	case "0000":
		status = model.StatusRejected
		reason := string(model.ReasonFondosInsuficientes)
		rejectionReason = &reason
		httpStatus = 402
	case "9999":
		status = model.StatusRejected
		reason := string(model.ReasonTarjetaExpiradaOBloqueada)
		rejectionReason = &reason
		httpStatus = 402
	case "1111":
		status = model.StatusRejected
		reason := string(model.ReasonSospechaDeFraude)
		rejectionReason = &reason
		httpStatus = 402
	default:
		status = model.StatusApproved
		code := generateAuthCode()
		authCode = &code
		httpStatus = 200
	}

	// 4. Crear entidad de transacción
	tx := &model.Transaction{
		ID:              uuid.NewString(),
		OrderID:         req.OrderID,
		Amount:          req.Amount,
		Currency:        currency,
		CardLastFour:    lastFour,
		Status:          status,
		AuthCode:        authCode,
		RejectionReason: rejectionReason,
		CreatedAt:       time.Now().UTC(),
	}

	// 5. Persistir en base de datos
	err = s.repo.Create(ctx, tx)
	if err != nil {
		// En caso de carrera concurrente con el mismo order_id
		if errors.Is(err, repository.ErrDuplicateOrder) {
			duplicateTx, getErr := s.repo.GetByOrderID(ctx, req.OrderID)
			if getErr == nil && duplicateTx != nil {
				log.Printf("[IDEMPOTENT_HIT] Concurrent duplicate for Order ID %s caught. Returning saved transaction %s", req.OrderID, duplicateTx.ID)
				resp, code := buildResponseFromTransaction(duplicateTx)
				return resp, code, nil
			}
		}
		return nil, 500, fmt.Errorf("failed to save transaction: %w", err)
	}

	resp, _ := buildResponseFromTransaction(tx)
	return resp, httpStatus, nil
}

func (s *paymentService) GetPaymentByOrderID(ctx context.Context, orderID string) (*model.TransactionDetailResponse, error) {
	if orderID == "" {
		return nil, ErrInvalidOrder
	}

	tx, err := s.repo.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	return &model.TransactionDetailResponse{
		TransactionID:   tx.ID,
		OrderID:         tx.OrderID,
		Amount:          tx.Amount,
		Currency:        tx.Currency,
		CardLastFour:    tx.CardLastFour,
		Status:          string(tx.Status),
		AuthCode:        tx.AuthCode,
		RejectionReason: tx.RejectionReason,
		CreatedAt:       tx.CreatedAt.Format(time.RFC3339),
	}, nil
}

func buildResponseFromTransaction(tx *model.Transaction) (*model.PaymentResponse, int) {
	resp := &model.PaymentResponse{
		TransactionID: tx.ID,
		OrderID:       tx.OrderID,
		Status:        string(tx.Status),
		CreatedAt:     tx.CreatedAt.Format(time.RFC3339),
	}

	if tx.Status == model.StatusApproved {
		resp.AuthCode = tx.AuthCode
		resp.Amount = &tx.Amount
		resp.Currency = &tx.Currency
		return resp, 200
	}

	resp.RejectionReason = tx.RejectionReason
	return resp, 402
}

func generateAuthCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return fmt.Sprintf("AUTH-%d", time.Now().UnixNano()%1000000)
	}
	return fmt.Sprintf("AUTH-%06d", n.Int64()+100000)
}
