package telegram

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cli-tg-chat-summary/internal/config"

	"github.com/celestix/gotgproto"
)

func TestTerminalAuthConversatorUsesProvidedInput(t *testing.T) {
	promptActive := &atomic.Bool{}
	promptWake := make(chan struct{}, 1)
	output := &strings.Builder{}
	conversator := newTerminalAuthConversator(strings.NewReader("12345\n"), output, promptActive, promptWake)

	code, err := conversator.AskCode()
	if err != nil {
		t.Fatalf("AskCode returned error: %v", err)
	}
	if code != "12345\n" {
		t.Fatalf("expected code from provided input, got %q", code)
	}
	if output.String() != "Enter Code: " {
		t.Fatalf("unexpected prompt output %q", output.String())
	}
	if promptActive.Load() {
		t.Fatal("expected prompt to be inactive after input is read")
	}
}

func TestStartClientTimeoutIgnoresActiveAuthPrompt(t *testing.T) {
	promptActive := &atomic.Bool{}
	promptWake := make(chan struct{}, 1)
	promptStarted := make(chan struct{})
	releasePrompt := make(chan struct{})
	client := &Client{
		cfg: &config.Config{
			TelegramConnectTimeoutSeconds: 1,
		},
		startProtoClient: func(_ *gotgproto.ClientOpts) (*gotgproto.Client, error) {
			promptActive.Store(true)
			wakePromptMonitor(promptWake)
			close(promptStarted)
			<-releasePrompt
			promptActive.Store(false)
			wakePromptMonitor(promptWake)
			return nil, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := client.startClient(&gotgproto.ClientOpts{Context: ctx}, cancel, promptActive, promptWake)
		done <- err
	}()

	select {
	case <-promptStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for auth prompt to start")
	}

	select {
	case err := <-done:
		t.Fatalf("startClient returned while auth prompt was active: %v", err)
	case <-time.After(1200 * time.Millisecond):
	}

	close(releasePrompt)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("startClient returned error after prompt completed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for startClient to finish")
	}
}

func TestStartClientTimesOutWhenNotPrompting(t *testing.T) {
	promptActive := &atomic.Bool{}
	promptWake := make(chan struct{}, 1)
	client := &Client{
		cfg: &config.Config{
			TelegramConnectTimeoutSeconds: 1,
		},
		startProtoClient: func(opts *gotgproto.ClientOpts) (*gotgproto.Client, error) {
			<-opts.Context.Done()
			return nil, opts.Context.Err()
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := client.startClient(&gotgproto.ClientOpts{Context: ctx}, cancel, promptActive, promptWake)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "telegram connection timed out after 1s") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("expected context to be canceled, got %v", ctx.Err())
	}
}

func wakePromptMonitor(promptWake chan<- struct{}) {
	select {
	case promptWake <- struct{}{}:
	default:
	}
}
