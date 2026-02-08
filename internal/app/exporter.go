package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cli-tg-chat-summary/internal/telegram"
)

type Exporter interface {
	Export(exportTitle string, messages []telegram.Message, opts RunOptions) (string, error)
}

type DefaultExporter struct {
	Now       func() time.Time
	Getwd     func() (string, error)
	MkdirAll  func(path string, perm os.FileMode) error
	Create    func(path string) (io.WriteCloser, error)
	Templates *TemplateRegistry
}

func NewDefaultExporter() *DefaultExporter {
	return &DefaultExporter{
		Now:       time.Now,
		Getwd:     os.Getwd,
		MkdirAll:  os.MkdirAll,
		Templates: NewDefaultTemplateRegistry(),
		Create: func(path string) (io.WriteCloser, error) {
			return os.Create(path)
		},
	}
}

func (e *DefaultExporter) Export(exportTitle string, messages []telegram.Message, opts RunOptions) (string, error) {
	registry := e.Templates
	if registry == nil {
		registry = NewDefaultTemplateRegistry()
	}
	formatName := strings.TrimSpace(opts.ExportFormat)
	if formatName == "" {
		formatName = "text"
	}
	formatName = strings.ToLower(formatName)
	template, ok := registry.Get(formatName)
	if !ok {
		return "", fmt.Errorf("unknown export format %q (available: %s)", formatName, strings.Join(registry.Names(), ", "))
	}

	// format: ChatName_Date.txt or ChatName_TopicName_Date.txt
	// date range format: ChatName_YYYY-MM-DD_to_YYYY-MM-DD.txt
	cleanName := sanitizeFilename(exportTitle)
	exportDate := e.Now()
	var suffix string
	if opts.UseDateRange {
		suffix = fmt.Sprintf("%s_to_%s", opts.Since.Format("2006-01-02"), opts.Until.Format("2006-01-02"))
	} else {
		suffix = exportDate.Format("2006-01-02")
	}
	filename := fmt.Sprintf("exports/%s_%s.%s", cleanName, suffix, template.Extension())

	cwd, err := e.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	if err := e.MkdirAll(filepath.Join(cwd, "exports"), 0755); err != nil {
		return "", fmt.Errorf("failed to create exports directory: %w", err)
	}

	fullPath := filepath.Join(cwd, filename)
	f, err := e.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}

	// Convert telegram messages to template messages.
	// Note: SenderName, ReplyTo, Reactions require additional API calls
	// and are not populated from the basic telegram.Message struct.
	templateMessages := make([]TemplateMessage, 0, len(messages))
	for _, msg := range messages {
		templateMessages = append(templateMessages, TemplateMessage{
			ID:       msg.ID,
			Date:     msg.Date,
			Text:     msg.Text,
			SenderID: msg.SenderID,
		})
	}
	input := TemplateInput{
		ExportTitle:   exportTitle,
		ExportDate:    exportDate,
		TotalMessages: len(messages),
		Messages:      templateMessages,
		Options:       opts,
	}
	if err := template.Render(f, input); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("failed to render %s: %w", template.Name(), err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("failed to close export file: %w", err)
	}
	return filename, nil
}
