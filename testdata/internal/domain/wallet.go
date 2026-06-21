package domain

// Wallet represents the wallet entity
type Wallet struct {
	ID      string
	Balance int64
}

// WalletRepository is the Interface to test the "IMPLEMENTS" feature of Lea
type WalletRepository interface {
	GetBalance(id string) (*Wallet, error)
	UpdateBalance(id string, amount int64) error
}
