package retry

import (
	"time"
)

func Retry(maxRetries int, initialDelay time.Duration, fn func() error) error {
	delay := initialDelay

	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {

		err = fn()
		if err == nil {
			return nil
		}

		if attempt == maxRetries {
			break
		}

		time.Sleep(delay)

		delay *= 2
	}

	return err
}

/* this can later be reused for:

Redis
Kafka
RabbitMQ
gRPC
External APIs
*/
