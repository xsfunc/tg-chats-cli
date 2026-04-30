package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/tgerr"
)

type testInvoker func(context.Context, bin.Encoder, bin.Decoder) error

func (f testInvoker) Invoke(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
	return f(ctx, input, output)
}

func TestFloodWaitMiddlewareStopsWhenWaitExceedsLimit(t *testing.T) {
	var recorded time.Duration
	middleware := newFloodWaitMiddleware(10*time.Second, func(d time.Duration) {
		recorded = d
	})

	err := middleware.Handle(testInvoker(func(context.Context, bin.Encoder, bin.Decoder) error {
		return tgerr.New(420, "FLOOD_WAIT_60")
	}))(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exceeds configured maximum") {
		t.Fatalf("unexpected error: %v", err)
	}
	if recorded != 60*time.Second {
		t.Fatalf("unexpected recorded wait: got %v want %v", recorded, 60*time.Second)
	}
}
