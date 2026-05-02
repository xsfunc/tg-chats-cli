package app

import (
	"context"

	"cli-tg-chat-summary/internal/telegram"
	"cli-tg-chat-summary/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

type fetchResult struct {
	result telegram.MessageFetchResult
	err    error
}

type FetchOpts struct {
	Ctx   context.Context
	Title string
}

type fetchHandle struct {
	msgCh    <-chan tea.Msg
	resultCh <-chan fetchResult
	cancel   context.CancelFunc
}

func (a *App) startFetchWithProgress(opts FetchOpts, fetch func(context.Context, telegram.ProgressFunc) (telegram.MessageFetchResult, error)) fetchHandle {
	msgCh := make(chan tea.Msg, 128)
	resultCh := make(chan fetchResult, 1)
	fetchCtx, cancel := context.WithCancel(opts.Ctx)

	go func() {
		progressFn := func(update telegram.ProgressUpdate) {
			if fetchCtx.Err() != nil {
				return
			}
			select {
			case msgCh <- tui.ProgressMsg{
				Phase:   update.Phase,
				Parsed:  update.Parsed,
				Scanned: update.Scanned,
				Batch:   update.Batch,
			}:
			default:
			}
		}

		result, err := fetch(fetchCtx, progressFn)
		resultCh <- fetchResult{result: result, err: err}
		close(msgCh)
	}()

	return fetchHandle{msgCh: msgCh, resultCh: resultCh, cancel: cancel}
}
