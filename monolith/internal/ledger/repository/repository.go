package repository

import (
	"context"
	"database/sql"
	"github.com/saurabhkr78/sudowallet/monolith/internal/ledger/model"
	"github.com/shopspring/decimal"
)

type LedgerRepository interface {
	//create and post a balance entry for a transaction in the ledger
	CreateTx(ctx context.Context, ledgerEntry *model.LedgerEntry, tx *sql.Tx) error
	// Get ledger history for an account.
	GetEntriesByWalletID(ctx context.Context, walletID string) ([]*model.LedgerEntry, error)
	// Get current balance.
	GetBalanceByWalletID(ctx context.Context, walletID string) (decimal.Decimal, error)
	// Reverse a previously posted transaction.
	//ReverseTransaction(...)

}
type mySQLLedgerRepository struct {
	db *sql.DB
}

func NewMySQLLedgerRepository(db *sql.DB) LedgerRepository {
	return &mySQLLedgerRepository{
		db: db,
	}
}

func (r *mySQLLedgerRepository) CreateTx(ctx context.Context, ledgerEntry *model.LedgerEntry, tx *sql.Tx) error {
	//all the fields of the ledgerEntry struct are required to be inserted into the ledger_entries table in the database
	query := "INSERT INTO ledger_entries (id,wallet_id,tranaction_id,amount,entry_type,balance,created_at) VALUES (?,?,?,?,?,?,?)"
	//now execute the query using the tx.ExecContext method and pass the context, query and the values of the ledgerEntry struct as arguments
	_, err := tx.ExecContext(ctx, query, ledgerEntry.ID, ledgerEntry.WalletID, ledgerEntry.TransactionID, ledgerEntry.Amount, ledgerEntry.EntryType, ledgerEntry.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *mySQLLedgerRepository) GetEntriesByWalletID(ctx context.Context, walletID string) ([]*model.LedgerEntry, error) {
	query := "SELECT id,wallet_id,transaction_id,amount,entry_type,balance,created_at FROM ledger_entries WHERE wallet_id = ?"
	//a wallet can have multiple ledger entries, so get all the rows from the ledger_entries table for the given wallet_id and return them as a slice of LedgerEntry structs
	//her rows is a pointer to sql.Rows, which is an iterator over the result set of the query. You can use rows.Next() to iterate over the rows and rows.Scan() to read the values of each row into variables.
	rows, err := r.db.QueryContext(ctx, query, walletID)
	if err != nil {
		return nil, err
	}
	//close the rows after we are done with them to free up the resources. This is important to avoid memory leaks and connection pool exhaustion.
	defer rows.Close()
	//now iterate over the rows and scan the values into a LedgerEntry struct and append it to a slice of LedgerEntry structs
	//to iterate over the result set u can use rows.Next() method which returns true if there is a next row and false if there are no more rows. You can use rows.Scan() method to scan the values of the current row into variables. You can use a for loop to iterate over the rows until rows.Next() returns false.
	var ledgerEntries []*model.LedgerEntry
	// how to itereate
	for rows.Next() {
		ledgerEntry := &model.LedgerEntry{}
		err := rows.Scan(&ledgerEntry.ID, &ledgerEntry.WalletID, &ledgerEntry.TransactionID, &ledgerEntry.Amount, &ledgerEntry.EntryType, &ledgerEntry.CreatedAt)
		if err != nil {
			return nil, err
		}
		ledgerEntries = append(ledgerEntries, ledgerEntry)
	}

	return ledgerEntries, nil
}

// if If your wallet's current balance is calculated from ledger entries, then the query is different else if your wallet's current balance is stored in the wallet table, then you can get the balance from there.
func (r *mySQLLedgerRepository) GetBalanceByWalletID(ctx context.Context, walletID string) (decimal.Decimal, error) {
	//so here we will assume that the wallet's current balance is calculated from ledger entries, so we will write a query to get the current balance from the ledger_entries table.
	//coalesce is used to return 0 if there are no entries for the wallet_id in the ledger_entries table to prevent null value from being returned and causing an error when trying to convert it to decimal.Decimal type.
	// Calculate the wallet balance from its ledger entries.
	// Credits increase the balance and debits decrease it.
	// COALESCE returns 0 when the wallet has no ledger entries,
	// because SUM() returns NULL when there are no rows.
	//viz,.Calculate the balance from these rows, and give me that calculated value as the result."
	query := `
		SELECT COALESCE(
    SUM(
        CASE
            WHEN entry_type = 'CREDIT' THEN amount
            WHEN entry_type = 'DEBIT' THEN -amount
            ELSE 0
        END
    ),
    0
)
FROM ledger_entries
WHERE wallet_id = ?`

	//sum the amount of all the credit entries and subtract the sum of all the debit entries for the given wallet_id to get the current balance of the wallet.
	var balance decimal.Decimal
	// QueryRowContext() gets the row. Scan() takes the columns from that row and puts them into your Go variables.
	//And for your balance query, there's only one column, so you only need: .Scan(&balance) since wallet ID is already provided in the query.
	// Pass the wallet ID as a query parameter so that only ledger entries
	// belonging to this wallet are included in the balance calculation.
	err := r.db.QueryRowContext(ctx, query, walletID).Scan(&balance)
	if err != nil {
		return decimal.Zero, err
	}
	return balance, nil
}
