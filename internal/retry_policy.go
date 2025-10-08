package internal

import "time"

type RetryPolicy struct {
	// Backoff interval for the first retry. If BackoffCoefficient is 1.0 then it is used for all retries.
	// If not set or set to 0, a default interval of 1s will be used.
	InitialInterval time.Duration

	// Coefficient used to calculate the next retry backoff interval.
	// The next retry interval is previous interval multiplied by this coefficient.
	// Must be 1 or larger. Default is 2.0.
	BackoffCoefficient float64

	// Maximum backoff interval between retries. Exponential backoff leads to interval increase.
	// This value is the cap of the interval. Default is 100x of initial interval.
	MaximumInterval time.Duration

	// Maximum number of attempts. When exceeded the retries stop even if not expired yet.
	// If not set or set to 0, it means unlimited, and rely on activity ScheduleToCloseTimeout to stop.
	MaximumAttempts int32

	// Non-Retriable errors. This is optional. Temporal server will stop retry if error type matches this list.
	//
	// Note:
	//  - cancellation is not a failure, so it won't be retried,
	//  - only StartToClose or Heartbeat timeouts are retryable.
	NonRetryableErrorTypes []string
}
