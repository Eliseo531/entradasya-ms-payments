package repository

import (
	"context"
	"sync"
	"time"

	"entradasya/ms-payments/internal/model"

	"github.com/google/uuid"
)

type MockPaymentRepository struct {
	mu           sync.RWMutex
	transactions map[string]*model.Transaction
}

func NewMockPaymentRepository() *MockPaymentRepository {
	return &MockPaymentRepository{
		transactions: make(map[string]*model.Transaction),
	}
}

func (m *MockPaymentRepository) GetByOrderID(_ context.Context, orderID string) (*model.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tx, exists := m.transactions[orderID]
	if !exists {
		return nil, ErrNotFound
	}

	// Retornamos una copia para evitar mutaciones externas
	copyTx := *tx
	return &copyTx, nil
}

func (m *MockPaymentRepository) Create(_ context.Context, tx *model.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.transactions[tx.OrderID]; exists {
		return ErrDuplicateOrder
	}

	if tx.ID == "" {
		tx.ID = uuid.NewString()
	}
	if tx.CreatedAt.IsZero() {
		tx.CreatedAt = time.Now().UTC()
	}

	copyTx := *tx
	m.transactions[tx.OrderID] = &copyTx
	return nil
}
