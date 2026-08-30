package scheduler

import (
	"context"
	"time"

	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

	"github.com/saurabhkr78/sudowallet/monolith/internal/logger"
	"github.com/shopspring/decimal"
)

func (s *Scheduler) CleanExpiredOTPs() {
	//TODO: Implement the logic to clean expired OTPs from the database
	// create a new context for this operation with a timeout of 5 seconds. if 5 seconds pass and the operation is not complete, it will be canceled to avoid hanging indefinitely. by defer cancel, we ensure that the context is canceled when the function returns, freeing up resources.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	//delete the expired otp from the database older than 1 hr ago
	query := "DELETE FROM otps WHERE created_at < NOW() - INTERVAL '1 hour'"
	//here by passing the context to ExecContext, we ensure that if the operation takes longer than 5 seconds, it will be canceled
	// now the query is context aware and will respect the timeout we set. if the operation takes longer than 5 seconds, it will be canceled, and an error will be returned.
	res, err := s.db.ExecContext(ctx, query)
	//now we log if table doesnot exist or any other error occurs
	if err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job] Failed to clean expired OTPs", "error", err.Error())
		return
	}
	//else we log the number of rows affected
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job] Failed to get affected rows", "error", err.Error())
		return
	}
	//log the number of rows affected
	logger.Log.InfoContext(ctx, "[Cron Job] Cleaned expired OTPs successfully..", "rows_affected", rowsAffected)
}

func (s *Scheduler) DailyAllBalanceReconciliation() {
	//TODO: Implement the logic to perform daily balance reconciliation
	//pre: get a  new context for this operation with 5 minutes time
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	//1. get all the accounts from the wallet table
	//couunt total accounts of wallet we are reconciling and how many of them have mismatch in balance with the ledger table
	query := "SELECT id,user_id, balance FROM wallets"
	rows, err := s.db.QueryContext(ctx, query)

	if err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job] Failed to get all accounts from wallet table", "error", err.Error())
		return
	}

	//close the rows after the function returns to avoid memory leaks
	defer rows.Close()

	totalAccountsInWallet := 0

	//2. calculate the balance for each account from the ledger table that is expected to be in the wallet table
	mismatchCount := 0
	totalAccountsInLedger := 0

	for rows.Next() {
		var walletID string
		var userID string
		var currwalletBalance decimal.Decimal

		if err := rows.Scan(&walletID, &userID, &currwalletBalance); err != nil {
			logger.Log.ErrorContext(ctx, "[Cron Job] Failed to scan wallet row", "error", err.Error())
			continue
		}

		totalAccountsInWallet++

		//calculate the expected balance from the ledger table for this walletID
		ledgerBalance, err := s.LedgerRepo.GetBalanceByWalletID(ctx, walletID)
		//count how many accounts of wallet we have checked
		if err != nil {
			logger.Log.ErrorContext(ctx, "[Cron Job] Failed to get ledger balance for walletID", "walletID", walletID, "error", err.Error())
			continue
		}

		totalAccountsInLedger++

		//3. compare the balance from the wallet table and the calculated balance from the ledger table
		if currwalletBalance.Cmp(ledgerBalance) != 0 {
			mismatchCount++

			logger.Log.WarnContext(
				ctx,
				"CRITICAL: BALANCE MISMATCH DETECTED",
				"walletID",
				walletID,
				"user_id",
				userID,
				"ledgerCalculatedBalance",
				ledgerBalance,
				"walletBalance",
				currwalletBalance,
				"difference",
				ledgerBalance.Sub(currwalletBalance),
			)

			//then in production, we can send an alert to the admin via email or slack or any other notification service. for now, we will just log it.
		}
	}

	if err := rows.Err(); err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job] Error while iterating wallet rows", "error", err.Error())
		return
	}

	//4. if there is a mismatch, log it and send an alert to the admin

	logger.Log.InfoContext(
		ctx,
		"[Cron Job] Performing daily balance reconciliation finished",
		"total_accounts_checked_from_wallet",
		totalAccountsInWallet,
		"total_accounts_checked_from_ledger",
		totalAccountsInLedger,
		"mismatches_found",
		mismatchCount,
	)
}

func (s *Scheduler) CleanExpiredRefreshTokens() {
	//create a new context for this operation with a timeout of 15 min
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	logger.Log.InfoContext(ctx, "[Cron Job]Cleaning expired refresh tokens started")
	//query to delete expired refresh tokens from the database older than 30 days
	query := "DELETE FROM refresh_tokens WHERE created_at < NOW()"

	//execute the query with the context to ensure it respects the timeout
	res, err := s.db.ExecContext(ctx, query)
	if err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job]Failed to clean expired refresh tokens", "error", err.Error())
		return
	}
	rowsAffected, _ := res.RowsAffected()
	logger.Log.InfoContext(ctx, "[Cron Job] Cleaned expired refresh tokens finished successfully.", "deleted_rows", rowsAffected)
}

func (s *Scheduler) ExportDailyTransactionReport() {
	//create a new context for this operation with a timeout of 30 min
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// a messagt to the terminal that the cron job has started
	logger.Log.InfoContext(ctx, "[Cron Job] Starting daily transaction report generation...")

	// query all the trxn of today from ledger table
	query := `SELECT id, sender_wallet_id, receiver_wallet_id, amount, status,
			created_at FROM transactions
			WHERE created_at >= CURRENT_DATE
			AND created_at < CURRENT_DATE + INTERVAL '1 day'`

	//execute the query with the context to ensure it respects the timeout
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job]Failed to export daily transaction report", "error", err.Error())
		return
	}
	defer rows.Close()

	//crete report directory folder to store the report if it does not exist
	reportDir := "./reports"
	if err := os.MkdirAll(reportDir, os.ModePerm); err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job]Failed to create report directory", "error", err.Error())
		return
	}

	//now create a csv file to naming structure and path to store  the report and the format shouold be daily_transaction_report_YYYY-MM-DD.csv
	reportFileName := filepath.Join(
		reportDir,
		fmt.Sprintf(
			"daily_transaction_report_%s.csv",
			time.Now().Format("20060102"),
		),
	)

	//now create a csv file to write the report
	file, err := os.Create(reportFileName)
	if err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job]Failed to create report file", "error", err.Error())
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	//write the csv column header to the csv file
	if err := writer.Write([]string{
		"Transaction ID",
		"Sender Wallet ID",
		"Receiver Wallet ID",
		"Amount",
		"Status",
		"Created At",
	}); err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job]Failed to write CSV header", "error", err.Error())
		return
	}

	rowCount := 0

	for rows.Next() {
		var id, receiverWalletID, status string
		var senderWalletID sql.NullString
		var amount decimal.Decimal
		var createdAt time.Time

		if err := rows.Scan(
			&id,
			&senderWalletID,
			&receiverWalletID,
			&amount,
			&status,
			&createdAt,
		); err != nil {
			logger.Log.ErrorContext(ctx, "[Cron Job]Failed to scan transaction row", "error", err.Error())
			continue
		}

		senderStr := ""
		if senderWalletID.Valid {
			senderStr = senderWalletID.String
		}

		if err := writer.Write([]string{
			id,
			senderStr,
			receiverWalletID,
			amount.StringFixed(2),
			status,
			createdAt.Format(time.RFC3339),
		}); err != nil {
			logger.Log.ErrorContext(ctx, "[Cron Job]Failed to write transaction to CSV", "error", err.Error())
			return
		}

		rowCount++
	}

	if err := rows.Err(); err != nil {
		logger.Log.ErrorContext(ctx, "[Cron Job]Failed while reading transactions", "error", err.Error())
		return
	}

	logger.Log.InfoContext(
		ctx,
		"[Cron Job] Daily transaction report generation finished successfully",
		"rows_written",
		rowCount,
		"file",
		reportFileName,
	)
}
