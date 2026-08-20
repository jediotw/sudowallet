/*

Now Go checks

Does *mysqlUserRepository have every method required by UserRepository?

Yes.

So automatically

*mysqlUserRepository
        │
implements
        ▼
UserRepository

Notice there is no keyword like Java's

implements

Go figures it out automatically.

This is called implicit interface implementation.


*/

package repository

import (
	"context"
	"database/sql"

	"errors"
	"github.com/saurabhkr78/sudowallet/monolith/internal/user/model"
)

type mysqlUserRepository struct {
	db *sql.DB
}

// the constructor creates the repository
func NewMySQLUserRepository(db *sql.DB) UserRepository {
	return &mysqlUserRepository{db: db}
}

func (r *mysqlUserRepository) Create(ctx context.Context, u *model.User) error {
	query := `INSERT INTO users(id,full_name,email,password_hash)VALUES(?,?,?,?)`
	_, err := r.db.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.PasswordHash)
	return err
}
func (r *mysqlUserRepository) GetById(ctx context.Context, id string) (*model.User, error) {
	query := `SELECT id, full_name, email,password_hash,created_at, updated_at, 
		deleted_at FROM users WHERE id = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.FullName, &u.Email, &u.PasswordHash,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return u, nil

}
func (r *mysqlUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `SELECT id, full_name, email, password_hash,created_at, updated_at, deleted_at FROM users WHERE email = ? AND deleted_at IS NULL`
	u := &model.User{}

	err := r.db.QueryRowContext(ctx, query, email).Scan(&u.ID, &u.FullName, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}

		return nil, err
	}

	return u, nil
}
func (r *mysqlUserRepository) Update(ctx context.Context, u *model.User) error {
	query := `UPDATE users SET full_name = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, u.FullName, u.ID)
	return err
}
func (r *mysqlUserRepository) CreateTx(ctx context.Context, u *model.User, tx *sql.Tx) error {
	query := `INSERT INTO users(id,full_name,email,password_hash)VALUES(?,?,?,?)`
	_, err := tx.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.PasswordHash)
	return err
}
