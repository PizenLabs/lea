package repository

import (
	"errors"
	"gopump/internal/domain"
)

// InMemWalletRepository implements domain.WalletRepository
type InMemWalletRepository struct {
	store map[string]*domain.Wallet
}

func NewInMem() domain.WalletRepository {
	return &InMemWalletRepository{
		store: make(map[string]*domain.Wallet),
	}
}

func (r *InMemWalletRepository) GetBalance(id string) (*domain.Wallet, error) {
	w, exists := r.store[id]
	if !exists {
		return nil, errors.New("wallet_not_found")
	}
	return w, nil
}

func (r *InMemWalletRepository) UpdateBalance(id string, amount int64) error {
	if w, exists := r.store[id]; exists {
		w.Balance += amount
		return nil
	}
	return errors.New("wallet_not_found")
}
