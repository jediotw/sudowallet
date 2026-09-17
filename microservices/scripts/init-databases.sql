-- One database per service. Executed by the mysql image on first boot
-- via /docker-entrypoint-initdb.d/.
CREATE DATABASE IF NOT EXISTS sudowallet_user;
CREATE DATABASE IF NOT EXISTS sudowallet_auth;
CREATE DATABASE IF NOT EXISTS sudowallet_wallet;
CREATE DATABASE IF NOT EXISTS sudowallet_transaction;
CREATE DATABASE IF NOT EXISTS sudowallet_payment;

-- MYSQL_USER is created by the official image; grant it access to every
-- service database so migrations and app connections can use one user.
GRANT ALL PRIVILEGES ON sudowallet_user.* TO 'sudowallet_user'@'%';
GRANT ALL PRIVILEGES ON sudowallet_auth.* TO 'sudowallet_user'@'%';
GRANT ALL PRIVILEGES ON sudowallet_wallet.* TO 'sudowallet_user'@'%';
GRANT ALL PRIVILEGES ON sudowallet_transaction.* TO 'sudowallet_user'@'%';
GRANT ALL PRIVILEGES ON sudowallet_payment.* TO 'sudowallet_user'@'%';

FLUSH PRIVILEGES;