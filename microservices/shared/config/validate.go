package config

import "errors"

func (h HTTPConfig) validate() error {
	if h.Port == "" {
		return errors.New("PORT is required")
	}

	return nil
}

func (d DBConfig) validate() error {
	if d.Host == "" {
		return errors.New("DB_HOST is required")
	}

	if d.Port == "" {
		return errors.New("DB_PORT is required")
	}

	if d.User == "" {
		return errors.New("DB_USER is required")
	}

	if d.Password == "" {
		return errors.New("DB_PASSWORD is required")
	}

	if d.Name == "" {
		return errors.New("DB_NAME is required")
	}

	if d.MaxRetries <= 0 {
		return errors.New("DB_MAX_RETRIES must be greater than 0")
	}

	if d.RetryDelay <= 0 {
		return errors.New("DB_RETRY_DELAY must be greater than 0")
	}

	if d.MaxOpenConns <= 0 {
		return errors.New("DB_MAX_OPEN_CONNS must be greater than 0")
	}

	if d.MaxIdleConns <= 0 {
		return errors.New("DB_MAX_IDLE_CONNS must be greater than 0")
	}

	if d.ConnMaxLifetime <= 0 {
		return errors.New("DB_CONN_MAX_LIFETIME must be greater than 0")
	}

	if d.ConnMaxIdleTime <= 0 {
		return errors.New("DB_CONN_MAX_IDLE_TIME must be greater than 0")
	}

	return nil
}

func (j JWTConfig) validate() error {
	if j.Secret == "" {
		return errors.New("JWT_SECRET is required")
	}

	return nil
}

func (c *Config) Validate() error {
	if err := c.HTTP.validate(); err != nil {
		return err
	}

	if err := c.DB.validate(); err != nil {
		return err
	}

	if err := c.JWT.validate(); err != nil {
		return err
	}

	return nil
}
