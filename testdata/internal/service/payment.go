package service

import (
	"fmt"
	"gopump/internal/domain"
	"gopump/pkg/logger"
)

type PaymentService struct {
	repo domain.WalletRepository // Depends on the Interface
	log  *logger.Logger
}

func NewPayment(repo domain.WalletRepository, log *logger.Logger) *PaymentService {
	return &PaymentService{repo: repo, log: log}
}

// ProcessDeposit is the function that executes the call chain (Call Chain) to test `lea trace` or `lea flow`
func (s *PaymentService) ProcessDeposit(walletID string, amount int64) error {
	s.log.Info(fmt.Sprintf("Starting deposit processing for wallet: %s", walletID))

	// Call flow: Service -> Repo (Interface)
	err := s.repo.UpdateBalance(walletID, amount)
	if err != nil {
		s.log.Info("Deposit failed")
		return err
	}

	s.log.Info("Deposit succeeded")
	return nil
}
