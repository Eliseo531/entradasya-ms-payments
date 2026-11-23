package model

import "time"

type TransactionStatus string

const (
	StatusApproved TransactionStatus = "APPROVED"
	StatusRejected TransactionStatus = "REJECTED"
)

type RejectionReason string

const (
	ReasonFondosInsuficientes       RejectionReason = "FONDOS_INSUFICIENTES"
	ReasonTarjetaExpiradaOBloqueada RejectionReason = "TARJETA_EXPIRADA_O_BLOQUEADA"
	ReasonSospechaDeFraude          RejectionReason = "SOSPECHA_DE_FRAUDE"
)

// Transaction representa la fila en la tabla transactions
type Transaction struct {
	ID              string            `json:"id" db:"id"`
	OrderID         string            `json:"order_id" db:"order_id"`
	Amount          float64           `json:"amount" db:"amount"`
	Currency        string            `json:"currency" db:"currency"`
	CardLastFour    string            `json:"card_last_four" db:"card_last_four"`
	Status          TransactionStatus `json:"status" db:"status"`
	AuthCode        *string           `json:"auth_code,omitempty" db:"auth_code"`
	RejectionReason *string           `json:"rejection_reason,omitempty" db:"rejection_reason"`
	CreatedAt       time.Time         `json:"created_at" db:"created_at"`
}

// PaymentRequest representa el payload recibido en POST /api/v1/payments
type PaymentRequest struct {
	OrderID    string  `json:"order_id" binding:"required"`
	Amount     float64 `json:"amount" binding:"required"`
	Currency   string  `json:"currency"`
	CardNumber string  `json:"card_number" binding:"required"`
	CardHolder string  `json:"card_holder"`
	ExpiryDate string  `json:"expiry_date"`
	CVV        string  `json:"cvv"`
}

// PaymentResponse representa la respuesta devuelta por POST /api/v1/payments
type PaymentResponse struct {
	TransactionID   string   `json:"transaction_id"`
	OrderID         string   `json:"order_id"`
	Status          string   `json:"status"`
	AuthCode        *string  `json:"auth_code,omitempty"`
	RejectionReason *string  `json:"rejection_reason,omitempty"`
	Amount          *float64 `json:"amount,omitempty"`
	Currency        *string  `json:"currency,omitempty"`
	CreatedAt       string   `json:"created_at"`
}

// TransactionDetailResponse representa la respuesta de GET /api/v1/payments/order/:orderId
type TransactionDetailResponse struct {
	TransactionID   string   `json:"transaction_id"`
	OrderID         string   `json:"order_id"`
	Amount          float64  `json:"amount"`
	Currency        string   `json:"currency"`
	CardLastFour    string   `json:"card_last_four"`
	Status          string   `json:"status"`
	AuthCode        *string  `json:"auth_code,omitempty"`
	RejectionReason *string  `json:"rejection_reason,omitempty"`
	CreatedAt       string   `json:"created_at"`
}
