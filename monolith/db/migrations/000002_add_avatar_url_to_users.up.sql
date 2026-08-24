ALTER TABLE users
ADD COLUMN avatar_url VARCHAR(255) NULL
AFTER password_hash;

-- AFTER password_hash means avatar_url is placed after password_hash.