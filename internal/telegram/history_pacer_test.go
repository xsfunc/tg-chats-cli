package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"tg-arc/internal/config"
)

func TestHistoryPacerWaitUsesConfiguredRange(t *testing.T) {
	pacer := newHistoryPacer(&config.Config{
		HistoryDelayMinMs: 2000,
		HistoryDelayMaxMs: 4000,
	})
	pacer.rand = func(n int) int {
		return n - 1
	}

	var slept time.Duration
	pacer.sleep = func(_ context.Context, d time.Duration) error {
		slept = d
		return nil
	}

	err := pacer.Wait(context.Background(), nil)
	if err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	if slept != 4*time.Second {
		t.Fatalf("unexpected sleep: got %v want %v", slept, 4*time.Second)
	}
}

func TestHistoryPacerBackoffIncreasesDelayRange(t *testing.T) {
	pacer := newHistoryPacer(&config.Config{
		HistoryDelayMinMs: 2000,
		HistoryDelayMaxMs: 4000,
	})
	pacer.rand = func(int) int {
		return 0
	}

	var slept []time.Duration
	pacer.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}

	// Drive the pacer from level 0 through 5 (max), then verify it is capped.
	// With base min=2s and rand=0 the sleep equals min*2^level.
	levels := maxHistoryBackoffLevel + 1 // one extra to verify cap
	for i := 0; i < levels; i++ {
		pacer.RecordFloodWait()
		if err := pacer.Wait(context.Background(), nil); err != nil {
			t.Fatalf("Wait level %d error: %v", i+1, err)
		}
	}

	// Expected min delays: 2s * 2^1=4, *4=8, *8=16, *16=32, *32=64, *32=64 (capped)
	want := make([]time.Duration, levels)
	for i := range want {
		factor := 1 << min(i+1, maxHistoryBackoffLevel)
		want[i] = time.Duration(factor) * 2 * time.Second
	}
	if len(slept) != len(want) {
		t.Fatalf("unexpected sleep count: got %d want %d", len(slept), len(want))
	}
	for i := range want {
		if slept[i] != want[i] {
			t.Fatalf("sleep %d: got %v want %v", i, slept[i], want[i])
		}
	}
}

func TestHistoryPacerBackoffReducesAfterSuccessfulPages(t *testing.T) {
	pacer := newHistoryPacer(&config.Config{
		HistoryDelayMinMs: 2000,
		HistoryDelayMaxMs: 4000,
	})
	pacer.rand = func(int) int {
		return 0
	}

	var slept []time.Duration
	pacer.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}

	pacer.RecordFloodWait()
	pacer.RecordFloodWait()

	for i := 0; i < successesToReduceDelay; i++ {
		pacer.RecordSuccess()
	}
	if err := pacer.Wait(context.Background(), nil); err != nil {
		t.Fatalf("Wait after first recovery error: %v", err)
	}

	for i := 0; i < successesToReduceDelay; i++ {
		pacer.RecordSuccess()
	}
	if err := pacer.Wait(context.Background(), nil); err != nil {
		t.Fatalf("Wait after second recovery error: %v", err)
	}

	want := []time.Duration{4 * time.Second, 2 * time.Second}
	for i := range want {
		if slept[i] != want[i] {
			t.Fatalf("sleep %d: got %v want %v", i, slept[i], want[i])
		}
	}
}

func TestHistoryPacerWaitHonorsContextCancellation(t *testing.T) {
	pacer := newHistoryPacer(&config.Config{
		HistoryDelayMinMs: 1000,
		HistoryDelayMaxMs: 1000,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := pacer.Wait(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestHistoryPacerLongFloodWaitSetsMaxBackoff(t *testing.T) {
	pacer := newHistoryPacer(&config.Config{
		HistoryDelayMinMs: 2000,
		HistoryDelayMaxMs: 4000,
	})
	pacer.rand = func(int) int { return 0 }

	var slept []time.Duration
	pacer.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}

	// A flood wait >= longFloodWaitThreshold must jump straight to maximum backoff.
	pacer.RecordFloodWaitDuration(5 * time.Minute)
	if err := pacer.Wait(context.Background(), nil); err != nil {
		t.Fatalf("Wait error: %v", err)
	}

	// max level = 5, factor = 2^5 = 32, min delay = 2s * 32 = 64s
	want := time.Duration(1<<maxHistoryBackoffLevel) * 2 * time.Second
	if len(slept) != 1 || slept[0] != want {
		t.Fatalf("expected sleep %v after long flood wait, got %v", want, slept)
	}
}

func TestHistoryPacerShortFloodWaitIncrementsLevelByOne(t *testing.T) {
	pacer := newHistoryPacer(&config.Config{
		HistoryDelayMinMs: 2000,
		HistoryDelayMaxMs: 4000,
	})
	pacer.rand = func(int) int { return 0 }

	var slept []time.Duration
	pacer.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}

	// A flood wait below the threshold increments by exactly one level.
	pacer.RecordFloodWaitDuration(30 * time.Second)
	if err := pacer.Wait(context.Background(), nil); err != nil {
		t.Fatalf("Wait error: %v", err)
	}

	// level 1: min delay = 2s * 2 = 4s
	if len(slept) != 1 || slept[0] != 4*time.Second {
		t.Fatalf("expected sleep 4s after short flood wait, got %v", slept)
	}
}
