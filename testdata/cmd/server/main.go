package main

import (
	"fmt"
	"gopump/internal/repository"
	"gopump/internal/service"
	"gopump/pkg/logger"
)

func main() {
	fmt.Println("Starting GoPump system...")

	// Initialize components (Dependency Injection)
	log := logger.New()
	repo := repository.NewInMem()
	payService := service.NewPayment(repo, log)

	// Call the real function to generate the Call Graph
	_ = payService.ProcessDeposit("wallet_123", 50000)
}
