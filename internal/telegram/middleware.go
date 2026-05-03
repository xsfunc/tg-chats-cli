package telegram

import (
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

type MiddlewareFunc func(next tg.Invoker) gotdtelegram.InvokeFunc

func (m MiddlewareFunc) Handle(next tg.Invoker) gotdtelegram.InvokeFunc {
	return m(next)
}
