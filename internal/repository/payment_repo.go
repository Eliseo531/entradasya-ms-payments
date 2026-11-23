package repository

import (
	"context"
	"errors"
	"fmt"

	"entradasya/ms-payments/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("transaction not found")
	ErrDuplicateOrder = errors.New("duplicate order id: transaction already exists")
)

type PaymentRepository interface {
	GetByOrderID(ctx context.Context, orderID string) (*model.Transaction, error)
	Create(ctx context.Context, tx *model.Transaction) error
}

type PostgresPaymentRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresPaymentRepository(pool *pgxpool.Pool) *PostgresPaymentRepository {
	return &PostgresPaymentRepository{pool: pool}
}

func (r *PostgresPaymentRepository) GetByOrderID(ctx context.Context, orderID string) (*model.Transaction, error) {
	query := `
		SELECT id, order_id, amount, currency, card_last_four, status, auth_code, rejection_reason, created_at
		FROM transactions
		WHERE order_id = $1
		LIMIT 1;
	`

	var tx model.Transaction
	err := r.pool.QueryRow(ctx, query, orderID).Scan(
		&tx.ID,
		&tx.OrderID,
		&tx.Amount,
		&tx.Currency,
		&tx.CardLastFour,
		&tx.Status,
		&tx.AuthCode,
		&tx.RejectionReason,
		&tx.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error querying transaction by order_id: %w", err)
	}

	return &tx, nil
}

func (r *PostgresPaymentRepository) Create(ctx context.Context, tx *model.Transaction) error {
	query := `
		INSERT INTO transactions (id, order_id, amount, currency, card_last_four, status, auth_code, rejection_reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at;
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		tx.ID,
		tx.OrderID,
		tx.Amount,
		tx.Currency,
		tx.CardLastFour,
		tx.Status,
		tx.AuthCode,
		tx.RejectionReason,
		tx.CreatedAt,
	).Scan(&tx.ID, &tx.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateOrder
		}
		return fmt.Errorf("error inserting transaction: %w", err)
	}

	return nil
}
