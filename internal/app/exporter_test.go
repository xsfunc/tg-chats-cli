package app

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"cli-tg-chat-summary/internal/telegram"
)

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error { return nil }

type errWriteCloser struct {
	io.Writer
	closeErr error
}

func (e errWriteCloser) Close() error { return e.closeErr }

type testExporterEnv struct {
	Exporter    *DefaultExporter
	Buffer      *bytes.Buffer
	CreatedPath string
	MkdirPath   string
	Cwd         string
}

func newTestExporterEnv(now time.Time) *testExporterEnv {
	env := &testExporterEnv{
		Buffer: &bytes.Buffer{},
		Cwd:    "/work",
	}
	env.Exporter = &DefaultExporter{
		Now:       func() time.Time { return now },
		Getwd:     func() (string, error) { return env.Cwd, nil },
		Templates: NewDefaultTemplateRegistry(),
		MkdirAll: func(path string, _ os.FileMode) error {
			env.MkdirPath = path
			return nil
		},
		Create: func(path string) (io.WriteCloser, error) {
			env.CreatedPath = path
			return nopWriteCloser{env.Buffer}, nil
		},
	}
	return env
}

func TestDefaultExporter_Export(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	env := newTestExporterEnv(now)

	messages := []telegram.Message{
		{SenderID: 10, Date: now, Text: "hello"},
		{SenderID: 10, Date: now.Add(1 * time.Minute), Text: "world"},
	}

	filename, err := env.Exporter.Export("My Chat", messages, RunOptions{})
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	if filename != "exports/My Chat_2025-01-02.txt" {
		t.Fatalf("unexpected filename: %s", filename)
	}
	if env.MkdirPath != "/work/exports" {
		t.Fatalf("unexpected mkdir path: %s", env.MkdirPath)
	}
	if env.CreatedPath != "/work/exports/My Chat_2025-01-02.txt" {
		t.Fatalf("unexpected created path: %s", env.CreatedPath)
	}

	output := env.Buffer.String()
	if !strings.Contains(output, "Chat Summary: My Chat\n") {
		t.Fatalf("missing header: %q", output)
	}
	if !strings.Contains(output, "Export Date: Thu, 02 Jan 2025 03:04:05 UTC\n") {
		t.Fatalf("missing export date: %q", output)
	}
	if !strings.Contains(output, "Total Messages: 2\n\n") {
		t.Fatalf("missing message count: %q", output)
	}
	if !strings.Contains(output, "[03:04-03:05] id=10:\n  hello\n  world\n") {
		t.Fatalf("missing message block: %q", output)
	}
}

func TestDefaultExporter_Export_DateRange(t *testing.T) {
	now := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	env := newTestExporterEnv(now)

	opts := RunOptions{
		UseDateRange: true,
		Since:        time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
		Until:        time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	filename, err := env.Exporter.Export("My Chat", nil, opts)
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	if filename != "exports/My Chat_2024-12-01_to_2024-12-31.txt" {
		t.Fatalf("unexpected date range filename: %s", filename)
	}
}

func TestDefaultExporter_Export_XML(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	env := newTestExporterEnv(now)

	messages := []telegram.Message{
		{SenderID: 10, Date: now, Text: "hello"},
		{SenderID: 10, Date: now.Add(1 * time.Minute), Text: "world"},
	}

	filename, err := env.Exporter.Export("My Chat", messages, RunOptions{ExportFormat: "xml"})
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	if filename != "exports/My Chat_2025-01-02.xml" {
		t.Fatalf("unexpected filename: %s", filename)
	}

	output := env.Buffer.String()
	if !strings.Contains(output, "<chat title=\"My Chat\">") {
		t.Fatalf("missing chat tag: %q", output)
	}
	if !strings.Contains(output, "<export_date>2025-01-02T03:04:05Z</export_date>") {
		t.Fatalf("missing export date: %q", output)
	}
	if !strings.Contains(output, "<total_messages>2</total_messages>") {
		t.Fatalf("missing total messages: %q", output)
	}
	if !strings.Contains(output, "<text>hello</text>") || !strings.Contains(output, "<text>world</text>") {
		t.Fatalf("missing message text: %q", output)
	}
}

func TestDefaultExporter_Export_XMLCompact(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	env := newTestExporterEnv(now)

	messages := []telegram.Message{
		{SenderID: 10, Date: now, Text: "hello"},
		{SenderID: 10, Date: now.Add(1 * time.Minute), Text: "world"},
	}

	filename, err := env.Exporter.Export("My Chat", messages, RunOptions{ExportFormat: "xml-compact"})
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	if filename != "exports/My Chat_2025-01-02.xml" {
		t.Fatalf("unexpected filename: %s", filename)
	}

	output := env.Buffer.String()
	if !strings.Contains(output, "<c t=\"My Chat\"") {
		t.Fatalf("missing compact chat tag: %q", output)
	}
	if !strings.Contains(output, "d=\"2025-01-02T03:04:05Z\"") {
		t.Fatalf("missing compact export date: %q", output)
	}
	if !strings.Contains(output, "n=\"2\"") {
		t.Fatalf("missing compact total messages: %q", output)
	}
	if !strings.Contains(output, "<m t=\"2025-01-02T03:04:05Z\" s=\"10\">hello</m>") {
		t.Fatalf("missing compact message text: %q", output)
	}
}

func TestDefaultExporter_Export_CloseError(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	closeErr := errors.New("disk full")
	buf := &bytes.Buffer{}

	exporter := &DefaultExporter{
		Now:       func() time.Time { return now },
		Getwd:     func() (string, error) { return "/work", nil },
		Templates: NewDefaultTemplateRegistry(),
		MkdirAll:  func(_ string, _ os.FileMode) error { return nil },
		Create: func(_ string) (io.WriteCloser, error) {
			return errWriteCloser{Writer: buf, closeErr: closeErr}, nil
		},
	}

	_, err := exporter.Export("Test", nil, RunOptions{})
	if err == nil {
		t.Fatal("expected error from Close(), got nil")
	}
	if !strings.Contains(err.Error(), "failed to close export file") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("error should wrap closeErr: %v", err)
	}
}
