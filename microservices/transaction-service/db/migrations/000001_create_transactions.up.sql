CREATE TABLE IF NOT EXISTS transactions (
    id VARCHAR(36) PRIMARY KEY,
    sender_wallet_id VARCHAR(36) NULL,
    receiver_wallet_id VARCHAR(36) NOT NULL,
    amount DECIMAL(15, 2) NOT NULL,
    description TEXT NULL,
    idempotency_key VARCHAR(100) UNIQUE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'success', -- success, pending, failed
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (sender_wallet_id) REFERENCES wallets(id),
    FOREIGN KEY (receiver_wallet_id) REFERENCES wallets(id)
);