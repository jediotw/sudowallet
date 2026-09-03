ALTER TABLE otp_codes
ADD COLUMN type VARCHAR(32) NOT NULL DEFAULT 'email_verification' AFTER code;