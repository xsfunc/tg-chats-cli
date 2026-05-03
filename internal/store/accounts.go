package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cli-tg-chat-summary/internal/telegram"
)

func (s *SQLiteStore) SetAccount(ctx context.Context, account telegram.Account) error {
	if account.TelegramUserID == 0 {
		return fmt.Errorf("telegram account id is required")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM accounts WHERE telegram_user_id = ?`, account.TelegramUserID).Scan(&id)
	if err == nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE accounts
			SET username = ?, first_name = ?, last_name = ?, phone = ?, is_bot = ?, updated_at = ?
			WHERE id = ?`,
			account.Username,
			account.FirstName,
			account.LastName,
			account.Phone,
			boolInt(account.IsBot),
			now,
			id,
		); err != nil {
			return fmt.Errorf("update account %d: %w", account.TelegramUserID, err)
		}
		s.accountID = id
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("find account %d: %w", account.TelegramUserID, err)
	}

	adoptedID, err := s.adoptLegacyAccount(ctx, account, now)
	if err != nil {
		return err
	}
	if adoptedID != 0 {
		s.accountID = adoptedID
		return nil
	}

	res, err := s.db.ExecContext(ctx, `INSERT INTO accounts
		(telegram_user_id, username, first_name, last_name, phone, is_bot, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		account.TelegramUserID,
		account.Username,
		account.FirstName,
		account.LastName,
		account.Phone,
		boolInt(account.IsBot),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("create account %d: %w", account.TelegramUserID, err)
	}
	id, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("read account id: %w", err)
	}
	s.accountID = id
	return nil
}

func (s *SQLiteStore) adoptLegacyAccount(ctx context.Context, account telegram.Account, now string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM accounts WHERE telegram_user_id IS NULL ORDER BY id LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("find legacy account: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE accounts
		SET telegram_user_id = ?, username = ?, first_name = ?, last_name = ?, phone = ?, is_bot = ?, updated_at = ?
		WHERE id = ? AND telegram_user_id IS NULL`,
		account.TelegramUserID,
		account.Username,
		account.FirstName,
		account.LastName,
		account.Phone,
		boolInt(account.IsBot),
		now,
		id,
	); err != nil {
		return 0, fmt.Errorf("adopt legacy account %d: %w", id, err)
	}
	return id, nil
}

func (s *SQLiteStore) requireAccount() (int64, error) {
	if s.accountID == 0 {
		return 0, fmt.Errorf("storage account is not set")
	}
	return s.accountID, nil
}
