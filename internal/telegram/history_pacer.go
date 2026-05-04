package telegram

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"tg-arc/internal/config"
)

const (
	maxHistoryBackoffLevel = 2
	successesToReduceDelay = 10
)

type sleepFunc func(context.Context, time.Duration) error
type randomIntFunc func(int) int

type historyPacer struct {
	mu sync.Mutex

	baseMin time.Duration
	baseMax time.Duration

	backoffLevel int
	successes    int

	rand  randomIntFunc
	sleep sleepFunc
}

func newHistoryPacer(cfg *config.Config) *historyPacer {
	baseMin := time.Duration(cfg.HistoryDelayMinMs) * time.Millisecond
	baseMax := time.Duration(cfg.HistoryDelayMaxMs) * time.Millisecond
	if baseMin <= 0 {
		baseMin = 2 * time.Second
	}
	if baseMax <= 0 {
		baseMax = 4 * time.Second
	}
	if baseMin > baseMax {
		baseMin, baseMax = baseMax, baseMin
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &historyPacer{
		baseMin: baseMin,
		baseMax: baseMax,
		rand: func(n int) int {
			return r.Intn(n)
		},
		sleep: sleepContext,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *historyPacer) Wait(ctx context.Context, progress ProgressFunc) error {
	delay := p.nextDelay()
	reportProgress(progress, ProgressUpdate{
		Phase: fmt.Sprintf("pausing %.1fs before next history request", delay.Seconds()),
	})
	return p.sleep(ctx, delay)
}

func (p *historyPacer) nextDelay() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	factor := 1 << p.backoffLevel
	minDelay := p.baseMin * time.Duration(factor)
	maxDelay := p.baseMax * time.Duration(factor)
	if maxDelay <= minDelay {
		return minDelay
	}
	spread := maxDelay - minDelay
	return minDelay + time.Duration(p.rand(int(spread)+1))
}

func (p *historyPacer) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.backoffLevel == 0 {
		p.successes = 0
		return
	}

	p.successes++
	if p.successes >= successesToReduceDelay {
		p.backoffLevel--
		p.successes = 0
	}
}

func (p *historyPacer) RecordFloodWait() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.backoffLevel < maxHistoryBackoffLevel {
		p.backoffLevel++
	}
	p.successes = 0
}
