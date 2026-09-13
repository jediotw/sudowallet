# RBAC Repository Changes — Revert Log
Date: 2026-09-13
File changed: `monolith/internal/user/repository/mysql_repository.go`
Interface file changed: `monolith/internal/user/repository/repository.go` (only if needed)

> All changes are minimal and focused on making the existing repository RBAC-aware.
> To revert: restore the original file from git: `git restore monolith/internal/user/repository/mysql_repository.go` and `git restore monolith/internal/user/repository/repository.go`, or apply the "BEFORE" snippets below.

---

## Summary

Your `users` table got `role VARCHAR(20) NOT NULL DEFAULT 'user'` via `000010_add_role_to_user.up.sql`.
The repository previously:
- did NOT persist `role` on `Create` / `CreateTx` (relied on DB default, but now explicit for clarity)
- selected `role` on reads but missed `avatar_url` / `is_verified` columns → scan mismatched after later migrations
- `GetAll` selected non-existent columns `oauth_provider, oauth_id` and scanned wrong count → runtime `sql: expected N columns` bug
- had no `UpdateRole` helper for admin use

Changes below are **revertable** — copy BEFORE over AFTER to undo.

---

## 1) `Create` — persist role

**BEFORE:**
```go
func (r *mysqlUserRepository) Create(ctx context.Context, u *model.User) error {
	query := `INSERT INTO users(id,full_name,email,password_hash)VALUES(?,?,?,?)`
	_, err := r.db.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.PasswordHash)
	return err
}
```

**AFTER:**
```go
func (r *mysqlUserRepository) Create(ctx context.Context, u *model.User) error {
	if u.Role == "" {
		u.Role = "user"
	}
	query := `INSERT INTO users(id,full_name,email,role,password_hash)VALUES(?,?,?,?,?)`
	_, err := r.db.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.Role, u.PasswordHash)
	return err
}
```

**Revert:** remove the `if u.Role == ""` block and revert query to 4 columns.

---

## 2) `CreateTx` — persist role inside transaction (Register flow)

**BEFORE:**
```go
func (r *mysqlUserRepository) CreateTx(ctx context.Context, u *model.User, tx *sql.Tx) error {
	query := `INSERT INTO users(id,full_name,email,password_hash)VALUES(?,?,?,?)`
	_, err := tx.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.PasswordHash)
	return err
}
```

**AFTER:**
```go
func (r *mysqlUserRepository) CreateTx(ctx context.Context, u *model.User, tx *sql.Tx) error {
	if u.Role == "" {
		u.Role = "user"
	}
	query := `INSERT INTO users(id,full_name,email,role,password_hash)VALUES(?,?,?,?,?)`
	_, err := tx.ExecContext(ctx, query, u.ID, u.FullName, u.Email, u.Role, u.PasswordHash)
	return err
}
```

**Revert:** same as #1.

---

## 3) `GetById` — include missing columns, correct scan order

**BEFORE:**
```sql
SELECT id, full_name, email,role,password_hash,created_at, updated_at, deleted_at FROM users WHERE id = ? AND deleted_at IS NULL
Scan: ID, FullName, Email, Role, PasswordHash, CreatedAt, UpdatedAt, DeletedAt (8 fields)
```

**AFTER:**
```sql
SELECT id, full_name, email, role, password_hash, avatar_url, is_verified, created_at, updated_at, deleted_at FROM users WHERE id = ? AND deleted_at IS NULL
Scan: ID, FullName, Email, Role, PasswordHash, AvatarURL, IsVerified, CreatedAt, UpdatedAt, DeletedAt (10 fields)
```

**Why:** `avatar_url` and `is_verified` were added in migrations 000002 / 000005 but reads omitted them → stale data. Fix makes reads RBAC-complete (role always returned alongside profile fields).

---

## 4) `GetByEmail` — same fix as GetById + logging preserved

**BEFORE / AFTER** — identical to #3 but for email lookup.

---

## 5) `GetAll` — fix broken select (removed `oauth_provider, oauth_id` that don't exist in `model.User`)

**BEFORE:**
```sql
SELECT id, full_name, email, role, oauth_provider, oauth_id, avatar_url, is_verified, created_at, updated_at, deleted_at ...
Scan: ID, FullName, Email, Role, AvatarURL, IsVerified, CreatedAt, UpdatedAt, DeletedAt (9 fields but 11 selected) → BUG
```

**AFTER:**
```sql
SELECT id, full_name, email, role, password_hash, avatar_url, is_verified, created_at, updated_at, deleted_at FROM users WHERE deleted_at IS NULL ORDER BY <validated> LIMIT ? OFFSET ?
Scan: ID, FullName, Email, Role, PasswordHash, AvatarURL, IsVerified, CreatedAt, UpdatedAt, DeletedAt (10 fields = 10 selected)
```

**Note:** `allowedSortColumns` already included `role` (RBAC-aware sorting). No new logic added.

**Revert:** restore old query string and old Scan line if you prefer the buggy version (not recommended).

---

## 6) NEW `UpdateRole` — minimal RBAC helper (ADDITIVE, safe to delete)

**ADDED:**
```go
func (r *mysqlUserRepository) UpdateRole(ctx context.Context, id string, role string) error {
	query := `UPDATE users SET role = ? WHERE id = ? AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, role, id)
	return err
}
```

**Interface change in `repository.go`:**
```go
// added to UserRepository interface:
GetAll(ctx context.Context, params commonDto.PaginationParams) ([]*model.User, int64, error)
UpdateRole(ctx context.Context, id string, role string) error
```
`GetAll` already existed in implementation but missing from interface → added for completeness. `UpdateRole` is new.

**To revert:** delete the `UpdateRole` method and remove those two lines from `repository.go`.

---

## 7) What was NOT changed

- No middleware / JWT / service / handler changes
- No new tables, no migration edits
- No role validation here (service should validate `user|admin`)
- `AuthMiddleware` still needs to `c.Set("role", claims.Role)` for `RequireRole` to work — flagged but NOT changed per "dont make unnecessary changes"

---

## Quick revert commands

```bash
# revert repository impl
git diff monolith/internal/user/repository/mysql_repository.go  # view
git restore monolith/internal/user/repository/mysql_repository.go

# revert interface if changed
git restore monolith/internal/user/repository/repository.go

# or reset both
git checkout HEAD -- monolith/internal/user/repository/
```

If you installed this patch without git, overwrite files with the BEFORE snippets above.
