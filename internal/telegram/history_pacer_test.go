package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"cli-tg-chat-summary/internal/config"
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

	pacer.RecordFloodWait()
	if err := pacer.Wait(context.Background(), nil); err != nil {
		t.Fatalf("Wait level 1 error: %v", err)
	}
	pacer.RecordFloodWait()
	if err := pacer.Wait(context.Background(), nil); err != nil {
		t.Fatalf("Wait level 2 error: %v", err)
	}
	pacer.RecordFloodWait()
	if err := pacer.Wait(context.Background(), nil); err != nil {
		t.Fatalf("Wait capped level error: %v", err)
	}

	want := []time.Duration{4 * time.Second, 8 * time.Second, 8 * time.Second}
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
