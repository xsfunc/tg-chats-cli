package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gotd/td/bin"
	tgtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

type floodWaitMiddleware struct {
	maxWait time.Duration
	onWait  func(time.Duration)
}

func newFloodWaitMiddleware(maxWait time.Duration, onWait func(time.Duration)) *floodWaitMiddleware {
	return &floodWaitMiddleware{
		maxWait: maxWait,
		onWait:  onWait,
	}
}

func (m *floodWaitMiddleware) Handle(next tg.Invoker) tgtelegram.InvokeFunc {
	return func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
		for {
			err := next.Invoke(ctx, input, output)
			if err == nil {
				return nil
			}

			wait, ok := tgerr.AsFloodWait(err)
			if !ok {
				return err
			}
			if wait == 0 {
				wait = time.Second
			}
			if m.onWait != nil {
				m.onWait(wait)
			}
			if m.maxWait > 0 && wait > m.maxWait {
				return fmt.Errorf("telegram flood wait %s exceeds configured maximum %s: %w", wait, m.maxWait, err)
			}

			slog.Warn("Telegram flood wait; pausing before retry", "wait", wait)
			if err := sleepContext(ctx, wait); err != nil {
				return err
			}
		}
	}
}
