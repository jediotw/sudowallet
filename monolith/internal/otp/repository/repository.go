package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/saurabhkr78/sudowallet/monolith/internal/otp/model"
)

// OTPRepository defines all database operations related to OTPs.
//
// We have two types of operations:
//
//  1. Normal operations
//     → use *sql.DB
//     → operation runs independently
//
//  2. Transactional operations
//     → use *sql.Tx
//     → operation becomes part of an existing transaction
//
// Example:
//
//	Normal:
//	    db.ExecContext(...)
//
//	Transactional:
//	    tx.ExecContext(...)
//	    tx.QueryRowContext(...)
type OTPRepository interface {

	// Create a new OTP.
	// This is a normal DB operation.
	Create(ctx context.Context, otp *model.OTP) error

	// Get an active OTP without using a transaction.
	//
	// Used when we simply need to read an OTP and don't
	// need to lock the row.
	GetActiveOTP(
		ctx context.Context,
		userID string,
		code string,
		otpType string,
	) (*model.OTP, error)

	// Mark an OTP as used without a transaction.
	//
	// This executes independently using *sql.DB.
	MarkOTPAsUsed(
		ctx context.Context,
		id string,
	) error

	// Get an active OTP inside an existing transaction.
	//
	// FOR UPDATE locks the selected OTP row until the
	// transaction is committed or rolled back.
	//
	// This is useful during email verification because
	// two concurrent requests should not be able to
	// consume the same OTP.
	GetActiveOTPTx(
		ctx context.Context,
		tx *sql.Tx,
		userID string,
		code string,
		otpType string,
	) (*model.OTP, error)

	// Mark an OTP as used inside an existing transaction.
	//
	// The UPDATE becomes part of the transaction and will
	// only become permanent when tx.Commit() succeeds.
	//
	// If the transaction is rolled back, this update is
	// rolled back as well.
	MarkOTPAsUsedTx(
		ctx context.Context,
		tx *sql.Tx,
		id string,
	) error
}

// mysqlOTPRepository is the MySQL implementation
// of OTPRepository.
type mysqlOTPRepository struct {
	db *sql.DB
}

// NewMySQLOTPRepository creates a new MySQL OTP repository.
func NewMySQLOTPRepository(db *sql.DB) OTPRepository {
	return &mysqlOTPRepository{
		db: db,
	}
}

// Create inserts a new OTP into the database.
//
// This operation is NOT part of an explicit transaction.
// Therefore it uses *sql.DB.
func (r *mysqlOTPRepository) Create(
	ctx context.Context,
	otp *model.OTP,
) error {

	query := `
		INSERT INTO otp_codes
			(id, user_id, code, type, expires_at, created_at, used)
		VALUES
			(?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		otp.ID,
		otp.UserID,
		otp.Code,
		otp.Type,
		otp.ExpiresAt,
		otp.CreatedAt,
		otp.Used,
	)

	return err
}

// GetActiveOTP finds an OTP that:
//
//   - belongs to the given user
//   - matches the provided code
//   - matches the OTP type
//   - has not been used
//   - has not expired
//
// This is a normal SELECT and does NOT lock the row.
func (r *mysqlOTPRepository) GetActiveOTP(
	ctx context.Context,
	userID string,
	code string,
	otpType string,
) (*model.OTP, error) {

	query := `
		SELECT
			id,
			user_id,
			code,
			type,
			expires_at,
			created_at,
			used
		FROM otp_codes
		WHERE user_id = ?
		  AND code = ?
		  AND type = ?
		  AND used = false
		  AND expires_at > NOW()
	`

	otp := &model.OTP{}

	err := r.db.QueryRowContext(
		ctx,
		query,
		userID,
		code,
		otpType,
	).Scan(
		&otp.ID,
		&otp.UserID,
		&otp.Code,
		&otp.Type,
		&otp.ExpiresAt,
		&otp.CreatedAt,
		&otp.Used,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("no active OTP found")
		}

		return nil, err
	}

	return otp, nil
}

// MarkOTPAsUsed marks an OTP as used.
//
// Because we are using *sql.DB instead of *sql.Tx,
// this UPDATE is an independent database operation.
func (r *mysqlOTPRepository) MarkOTPAsUsed(
	ctx context.Context,
	id string,
) error {

	query := `
		UPDATE otp_codes
		SET used = true
		WHERE id = ?
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		id,
	)

	return err
}

// GetActiveOTPTx finds and LOCKS an active OTP row
// inside an existing transaction.
//
// Important:
//
//	SELECT ... FOR UPDATE
//
// means:
//
// "Select this row and acquire a row lock for this
// transaction. A conflicting UPDATE/DELETE/lock attempt
// from another transaction must wait until this transaction
// commits or rolls back."
//
// This is useful for OTP verification.
//
// Example:
//
//	BEGIN
//	    ↓
//	SELECT ... FOR UPDATE
//	    ↓
//	OTP row locked
//	    ↓
//	UPDATE OTP
//	    ↓
//	UPDATE USER
//	    ↓
//	COMMIT
//
// If something fails:
//
//	ROLLBACK
//
// and the OTP update + user update are both undone.
func (r *mysqlOTPRepository) GetActiveOTPTx(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	code string,
	otpType string,
) (*model.OTP, error) {

	query := `
		SELECT
			id,
			user_id,
			code,
			type,
			expires_at,
			created_at,
			used
		FROM otp_codes
		WHERE user_id = ?
		  AND code = ?
		  AND type = ?
		  AND expires_at > NOW()
		  AND used = false
		FOR UPDATE
	`

	otp := &model.OTP{}

	err := tx.QueryRowContext(
		ctx,
		query,
		userID,
		code,
		otpType,
	).Scan(
		&otp.ID,
		&otp.UserID,
		&otp.Code,
		&otp.Type,
		&otp.ExpiresAt,
		&otp.CreatedAt,
		&otp.Used,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("active OTP not found or expired")
		}

		return nil, err
	}

	return otp, nil
}

// MarkOTPAsUsedTx marks an OTP as used INSIDE
// an existing transaction.
//
// Notice that we use:
//
//	tx.ExecContext()
//
// instead of:
//
//	r.db.ExecContext()
//
// Because we want this UPDATE to be part of the
// same transaction that selected the OTP.
//
// The UPDATE is not permanently committed until:
//
//	tx.Commit()
//
// If:
//
//	tx.Rollback()
//
// happens, this UPDATE is undone.
func (r *mysqlOTPRepository) MarkOTPAsUsedTx(
	ctx context.Context,
	tx *sql.Tx,
	id string,
) error {

	query := `
		UPDATE otp_codes
		SET used = true
		WHERE id = ?
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		id,
	)

	return err
}
