package telegram

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/gotd/td/telegram/dcs"
	xproxy "golang.org/x/net/proxy"
)

func newProxyResolver(rawURL string) (dcs.Resolver, error) {
	if rawURL == "" {
		return nil, nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse telegram proxy url: %w", err)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("telegram proxy url must include host and port")
	}
	switch parsed.Scheme {
	case "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("unsupported telegram proxy scheme %q; use socks5://host:port", parsed.Scheme)
	}

	dialer, err := xproxy.FromURL(parsed, xproxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("create telegram proxy dialer: %w", err)
	}

	return dcs.Plain(dcs.PlainOptions{
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			if contextDialer, ok := dialer.(xproxy.ContextDialer); ok {
				return contextDialer.DialContext(ctx, network, address)
			}
			return dialWithContext(ctx, dialer, network, address)
		},
	}), nil
}

func dialWithContext(ctx context.Context, dialer xproxy.Dialer, network, address string) (net.Conn, error) {
	type dialResult struct {
		conn net.Conn
		err  error
	}

	resultCh := make(chan dialResult, 1)
	go func() {
		conn, err := dialer.Dial(network, address)
		if conn != nil && ctx.Err() != nil {
			_ = conn.Close()
		}
		resultCh <- dialResult{conn: conn, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		return result.conn, result.err
	}
}
