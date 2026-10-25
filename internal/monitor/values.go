package monitor

import "time"

// Monitor scheduling policies, independent of socket operation deadlines.
const (
	defaultRetryMin     = time.Second
	defaultRetryMax     = time.Minute
	defaultHealthyAfter = 2 * time.Minute
	defaultWriteTimeout = 10 * time.Second
	defaultPollInterval = time.Minute
	defaultPingInterval = 25 * time.Second
	retryMultiplier     = 2
	retryJitterDivisor  = 4
)
