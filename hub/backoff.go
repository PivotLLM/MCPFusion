/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"sync"
	"time"
)

const (
	defaultBaseDelay = 1 * time.Second
	defaultMaxDelay  = 60 * time.Second
	defaultFactor    = 2.0
	absoluteMaxDelay = 60 * time.Second
)

// ExponentialBackoff implements exponential backoff for hub service restarts.
// It multiplies the current delay by the configured factor after each wait,
// capping the delay at maxDelay.
type ExponentialBackoff struct {
	mu           sync.Mutex
	baseDelay    time.Duration
	maxDelay     time.Duration
	factor       float64
	currentDelay time.Duration
}

// NewExponentialBackoff creates a new ExponentialBackoff with the given parameters.
// Zero values are replaced with defaults: baseDelay=1s, maxDelay=60s, factor=2.0.
// maxDelay is capped at 60s regardless of the value provided.
func NewExponentialBackoff(baseDelay, maxDelay time.Duration, factor float64) *ExponentialBackoff {
	if baseDelay <= 0 {
		baseDelay = defaultBaseDelay
	}

	if maxDelay <= 0 {
		maxDelay = defaultMaxDelay
	}

	if maxDelay > absoluteMaxDelay {
		maxDelay = absoluteMaxDelay
	}

	if factor <= 0 {
		factor = defaultFactor
	}

	// Ensure baseDelay does not exceed maxDelay
	if baseDelay > maxDelay {
		baseDelay = maxDelay
	}

	return &ExponentialBackoff{
		baseDelay:    baseDelay,
		maxDelay:     maxDelay,
		factor:       factor,
		currentDelay: baseDelay,
	}
}

// Wait blocks for the current delay duration or until the context is cancelled.
// After waiting, the current delay is multiplied by the factor, capped at maxDelay.
// Returns ctx.Err() if the context is cancelled before the delay elapses, nil otherwise.
func (b *ExponentialBackoff) Wait(ctx context.Context) error {
	b.mu.Lock()
	delay := b.currentDelay
	b.mu.Unlock()

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	b.mu.Lock()
	b.currentDelay = time.Duration(float64(b.currentDelay) * b.factor)
	if b.currentDelay > b.maxDelay {
		b.currentDelay = b.maxDelay
	}
	b.mu.Unlock()

	return nil
}

// Reset resets the current delay back to the base delay.
func (b *ExponentialBackoff) Reset() {
	b.mu.Lock()
	b.currentDelay = b.baseDelay
	b.mu.Unlock()
}

// CurrentDelay returns the current delay duration.
func (b *ExponentialBackoff) CurrentDelay() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentDelay
}
