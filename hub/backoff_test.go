/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package hub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewExponentialBackoff_Defaults(t *testing.T) {
	b := NewExponentialBackoff(0, 0, 0)

	assert.Equal(t, defaultBaseDelay, b.baseDelay, "baseDelay should default to 1s")
	assert.Equal(t, defaultMaxDelay, b.maxDelay, "maxDelay should default to 60s")
	assert.Equal(t, defaultFactor, b.factor, "factor should default to 2.0")
	assert.Equal(t, defaultBaseDelay, b.currentDelay, "currentDelay should start at baseDelay")
}

func TestNewExponentialBackoff_MaxDelayCap(t *testing.T) {
	b := NewExponentialBackoff(1*time.Second, 120*time.Second, 2.0)

	assert.Equal(t, absoluteMaxDelay, b.maxDelay, "maxDelay exceeding 60s should be capped to 60s")
}

func TestExponentialBackoff_DelayProgression(t *testing.T) {
	base := 10 * time.Millisecond
	max := 100 * time.Millisecond
	b := NewExponentialBackoff(base, max, 2.0)

	expected := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
		80 * time.Millisecond,
	}

	ctx := context.Background()
	for i, want := range expected {
		assert.Equal(t, want, b.CurrentDelay(), "delay before wait %d", i)
		err := b.Wait(ctx)
		assert.NoError(t, err, "Wait should not return error on iteration %d", i)
	}
}

func TestExponentialBackoff_MaxDelayClamped(t *testing.T) {
	base := 10 * time.Millisecond
	max := 30 * time.Millisecond
	b := NewExponentialBackoff(base, max, 2.0)

	ctx := context.Background()

	// 10ms -> 20ms -> 30ms (clamped) -> 30ms (stays clamped)
	err := b.Wait(ctx) // waits 10ms, advances to 20ms
	assert.NoError(t, err)

	err = b.Wait(ctx) // waits 20ms, advances to 40ms but clamped to 30ms
	assert.NoError(t, err)

	assert.Equal(t, max, b.CurrentDelay(), "delay should be clamped to maxDelay")

	err = b.Wait(ctx) // waits 30ms, stays at 30ms
	assert.NoError(t, err)

	assert.Equal(t, max, b.CurrentDelay(), "delay should remain clamped at maxDelay")
}

func TestExponentialBackoff_Reset(t *testing.T) {
	base := 10 * time.Millisecond
	max := 100 * time.Millisecond
	b := NewExponentialBackoff(base, max, 2.0)

	ctx := context.Background()

	// Advance the delay a few times
	_ = b.Wait(ctx) // 10ms -> 20ms
	_ = b.Wait(ctx) // 20ms -> 40ms
	assert.Equal(t, 40*time.Millisecond, b.CurrentDelay(), "delay should have progressed")

	b.Reset()
	assert.Equal(t, base, b.CurrentDelay(), "Reset should restore delay to baseDelay")
}

func TestExponentialBackoff_ContextCancellation(t *testing.T) {
	base := 10 * time.Second // long delay to ensure we don't wait
	b := NewExponentialBackoff(base, 60*time.Second, 2.0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling Wait

	start := time.Now()
	err := b.Wait(ctx)
	elapsed := time.Since(start)

	assert.Error(t, err, "Wait should return an error when context is cancelled")
	assert.Equal(t, context.Canceled, err, "error should be context.Canceled")
	assert.Less(t, elapsed, 1*time.Second, "Wait should return immediately on cancelled context")
}

func TestExponentialBackoff_CustomFactor(t *testing.T) {
	base := 10 * time.Millisecond
	max := 100 * time.Millisecond
	factor := 3.0
	b := NewExponentialBackoff(base, max, factor)

	assert.Equal(t, factor, b.factor, "factor should be set to custom value")

	ctx := context.Background()

	// 10ms -> 30ms -> 90ms -> 100ms (clamped)
	expected := []time.Duration{
		10 * time.Millisecond,
		30 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	for i, want := range expected {
		assert.Equal(t, want, b.CurrentDelay(), "delay before wait %d with factor 3.0", i)
		err := b.Wait(ctx)
		assert.NoError(t, err, "Wait should not return error on iteration %d", i)
	}
}
