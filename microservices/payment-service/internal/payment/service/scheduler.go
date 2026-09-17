package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/saurabhkr78/sudowallet/microservices/payment-service/internal/payment/repository"
	"github.com/saurabhkr78/sudowallet/microservices/shared/logger"
)

// Scheduler owns the payment-side background jobs:
//   - DailyAllBalanceReconciliation  at 02:00
//   - ExportDailyTransactionReport   at 23:59
//
// The monolith's CleanExpiredOTPs job is dead code (the otps table was dropped)
// and CleanExpiredRefreshTokens moved to auth-service, so neither is repeated
// here.
type Scheduler struct {
	ledgerRepo      repository.LedgerRepository
	walletRepo      repository.WalletRepository
	transactionRepo repository.TransactionRepository
	reportDir       string
}

func NewScheduler(
	lRepo repository.LedgerRepository,
	wRepo repository.WalletRepository,
	txRepo repository.TransactionRepository,
	reportDir string,
) *Scheduler {
	return &Scheduler{
		ledgerRepo:      lRepo,
		walletRepo:      wRepo,
		transactionRepo: txRepo,
		reportDir:       reportDir,
	}
}

// Start schedules both daily jobs in their own goroutines.
func (s *Scheduler) Start() {
	go s.runDaily("DailyAllBalanceReconciliation", "02:00", s.DailyAllBalanceReconciliation)
	go s.runDaily("ExportDailyTransactionReport", "23:59", s.ExportDailyTransactionReport)
	logger.Log.Info("payment-service scheduler started")
}

// runDaily repeatedly sleeps until the next occurrence of hh:mm ("02:00") and
// then runs fn. It re-schedules itself after each run.
func (s *Scheduler) runDaily(name, hhmm string, fn func()) {
	for {
		next := nextFireTime(time.Now(), hhmm)
		time.Sleep(time.Until(next))
		logger.Log.Info("[Job] starting", "job", name)
		fn()
	}
}

func nextFireTime(now time.Time, hhmm string) time.Time {
	h, m := 0, 0
	_, _ = fmt.Sscanf(hhmm, "%02d:%02d", &h, &m)

	next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// DailyAllBalanceReconciliation compares every wallet balance with the balance
// recomputed from its ledger entries and logs any mismatch.
func (s *Scheduler) DailyAllBalanceReconciliation() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	wallets, err := s.walletRepo.GetAll(ctx)
	if err != nil {
		logger.Error(ctx, "[Cron Job] Failed to get wallets", "error", err.Error())
		return
	}

	totalAccountsInWallet := 0
	totalAccountsInLedger := 0
	mismatchCount := 0

	for _, wallet := range wallets {
		totalAccountsInWallet++

		ledgerBalance, err := s.ledgerRepo.GetBalanceByWalletID(ctx, wallet.ID)
		if err != nil {
			logger.Error(ctx, "[Cron Job] Failed to get ledger balance", "wallet_id", wallet.ID, "error", err.Error())
			continue
		}
		totalAccountsInLedger++

		if !wallet.Balance.Equal(ledgerBalance) {
			mismatchCount++
			logger.Warn(
				ctx,
				"CRITICAL: BALANCE MISMATCH DETECTED",
				"wallet_id", wallet.ID,
				"user_id", wallet.UserID,
				"ledger_calculated_balance", ledgerBalance,
				"wallet_balance", wallet.Balance,
				"difference", ledgerBalance.Sub(wallet.Balance),
			)
		}
	}

	logger.Info(
		ctx,
		"[Cron Job] Daily balance reconciliation finished",
		"total_accounts_checked_from_wallet", totalAccountsInWallet,
		"total_accounts_checked_from_ledger", totalAccountsInLedger,
		"mismatches_found", mismatchCount,
	)
}

// ExportDailyTransactionReport writes today's transactions to a CSV file under
// ./reports with the name daily_transaction_report_YYYYMMDD.csv.
func (s *Scheduler) ExportDailyTransactionReport() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	logger.Info(ctx, "[Cron Job] Starting daily transaction report generation...")

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)

	txs, err := s.transactionRepo.GetTransactionsByDate(ctx, start, end)
	if err != nil {
		logger.Error(ctx, "[Cron Job] Failed to fetch transactions", "error", err.Error())
		return
	}

	if err := os.MkdirAll(s.reportDir, os.ModePerm); err != nil {
		logger.Error(ctx, "[Cron Job] Failed to create report directory", "error", err.Error())
		return
	}

	reportFileName := filepath.Join(
		s.reportDir,
		fmt.Sprintf("daily_transaction_report_%s.csv", now.Format("20060102")),
	)

	file, err := os.Create(reportFileName)
	if err != nil {
		logger.Error(ctx, "[Cron Job] Failed to create report file", "error", err.Error())
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{
		"Transaction ID",
		"Sender Wallet ID",
		"Receiver Wallet ID",
		"Amount",
		"Status",
		"Created At",
	}); err != nil {
		logger.Error(ctx, "[Cron Job] Failed to write CSV header", "error", err.Error())
		return
	}

	for _, tx := range txs {
		if err := writer.Write([]string{
			tx.ID,
			tx.SenderWalletID,
			tx.ReceiverWalletID,
			tx.Amount.StringFixed(2),
			tx.Status,
			tx.CreatedAt.Format(time.RFC3339),
		}); err != nil {
			logger.Error(ctx, "[Cron Job] Failed to write row to CSV", "error", err.Error())
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		logger.Error(ctx, "[Cron Job] Failed to flush CSV", "error", err.Error())
		return
	}

	logger.Info(
		ctx,
		"[Cron Job] Daily transaction report generation finished",
		"rows_written", len(txs),
		"file", reportFileName,
	)
}
