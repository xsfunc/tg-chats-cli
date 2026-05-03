package store

import (
	"context"
	"fmt"
	"time"
)

func (s *SQLiteStore) StartRun(ctx context.Context, mode string) (SyncRun, error) {
	accountID, err := s.requireAccount()
	if err != nil {
		return SyncRun{}, err
	}
	started := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO sync_runs (account_id, mode, started_at, status) VALUES (?, ?, ?, ?)`, accountID, mode, started.Format(time.RFC3339Nano), "running")
	if err != nil {
		return SyncRun{}, fmt.Errorf("start sync run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return SyncRun{}, fmt.Errorf("read sync run id: %w", err)
	}
	return SyncRun{ID: id, Mode: mode, StartedAt: started}, nil
}

func (s *SQLiteStore) FinishRun(ctx context.Context, runID int64, status string, runErr error) error {
	accountID, err := s.requireAccount()
	if err != nil {
		return err
	}
	errText := ""
	if runErr != nil {
		errText = runErr.Error()
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE sync_runs SET finished_at = ?, status = ?, error = ? WHERE id = ? AND account_id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), status, errText, runID, accountID); err != nil {
		return fmt.Errorf("finish sync run %d: %w", runID, err)
	}
	return nil
}

func (s *SQLiteStore) AddRunItem(ctx context.Context, item RunItem) error {
	accountID, err := s.requireAccount()
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sync_run_items
		(account_id, run_id, chat_id, topic_id, saved_messages, mark_read_status, warning, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		accountID,
		item.RunID,
		item.ChatID,
		item.TopicID,
		item.SavedMessages,
		item.MarkReadStatus,
		item.Warning,
		item.Error,
	); err != nil {
		return fmt.Errorf("add sync run item: %w", err)
	}
	return nil
}
