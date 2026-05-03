package store

import (
	"database/sql"

	"cli-tg-chat-summary/internal/telegram"
)

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}

func closeStmt(stmt *sql.Stmt) {
	_ = stmt.Close()
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func chatKind(chat telegram.Chat) string {
	switch {
	case chat.IsUser:
		return "user"
	case chat.IsChannel:
		return "channel"
	default:
		return "chat"
	}
}
