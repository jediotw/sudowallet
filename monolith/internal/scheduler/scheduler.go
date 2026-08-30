package scheduler

import (
	"database/sql"

	"github.com/robfig/cron/v3"
	ledgerRepo "github.com/saurabhkr78/sudowallet/monolith/internal/ledger/repository"
	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	walletRepo "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/repository"
)

type Scheduler struct {
	cron       *cron.Cron
	db         *sql.DB
	walletRepo walletRepo.WalletRepository
	LedgerRepo ledgerRepo.LedgerRepository
}

func NewScheduler(db *sql.DB, wRepo walletRepo.WalletRepository, lRepo ledgerRepo.LedgerRepository) *Scheduler {
	c := cron.New(
		cron.WithSeconds(), // Enable seconds field
	)
	return &Scheduler{
		cron:       c,
		db:         db,
		walletRepo: wRepo,
		LedgerRepo: lRepo,
	}
}

func (s *Scheduler) Start() {
	//clean expired otp every 30 minutes
	s.cron.AddFunc("0 */30 * * * *", s.CleanExpiredOTPs)
	//daily balance reconciliation at 2:00 AM
	s.cron.AddFunc("0 0 2 * * *", s.DailyAllBalanceReconciliation)
	// clean expired refresh tokens at 3:00 AM daily
	s.cron.AddFunc("0 0 3 * * *", s.CleanExpiredRefreshTokens)
	//export daily transaction report at 23:59 PMdaily
	s.cron.AddFunc("0 59 23 * * *", s.ExportDailyTransactionReport)

	s.cron.Start()
	logger.Log.Info("Background Scheduler started sucessfully")
}
func (s *Scheduler) Stop() {
	s.cron.Stop()
	logger.Log.Info("Background Scheduler stopped sucessfully")
}
