package telegram

import "testing"

func TestNewProxyResolver(t *testing.T) {
	resolver, err := newProxyResolver("socks5://127.0.0.1:1080")
	if err != nil {
		t.Fatalf("newProxyResolver returned error: %v", err)
	}
	if resolver == nil {
		t.Fatal("expected resolver")
	}
}

func TestNewProxyResolverRejectsHTTP(t *testing.T) {
	_, err := newProxyResolver("http://127.0.0.1:8080")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewProxyResolverRequiresHost(t *testing.T) {
	_, err := newProxyResolver("socks5://")
	if err == nil {
		t.Fatal("expected error")
	}
}
