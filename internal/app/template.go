package app

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type Template interface {
	Name() string
	Extension() string
	Render(w io.Writer, input TemplateInput) error
}

type TemplateInput struct {
	ExportTitle   string
	ExportDate    time.Time
	TotalMessages int
	Messages      []TemplateMessage
	Options       RunOptions
}

type TemplateMessage struct {
	ID         int
	Date       time.Time
	Text       string
	SenderID   int64
	SenderName string             // Reserved: requires user lookup, not populated yet
	ReplyTo    *TemplateReply     // Reserved: requires additional API calls
	Reactions  []TemplateReaction // Reserved: requires additional API calls
}

type TemplateReply struct {
	MessageID  int
	SenderID   int64
	SenderName string
	Text       string
}

type TemplateReaction struct {
	Emoji string
	Count int
}

type TemplateRegistry struct {
	templates map[string]Template
}

func NewTemplateRegistry() *TemplateRegistry {
	return &TemplateRegistry{templates: make(map[string]Template)}
}

func (r *TemplateRegistry) Register(template Template) error {
	if template == nil {
		return fmt.Errorf("template is nil")
	}
	name := strings.TrimSpace(template.Name())
	if name == "" {
		return fmt.Errorf("template name is empty")
	}
	if _, exists := r.templates[name]; exists {
		return fmt.Errorf("template %q already registered", name)
	}
	r.templates[name] = template
	return nil
}

func (r *TemplateRegistry) Get(name string) (Template, bool) {
	template, ok := r.templates[name]
	return template, ok
}

func (r *TemplateRegistry) Names() []string {
	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func NewDefaultTemplateRegistry() *TemplateRegistry {
	registry := NewTemplateRegistry()
	if err := registry.Register(NewTextTemplate()); err != nil {
		panic(err)
	}
	if err := registry.Register(NewXMLTemplate()); err != nil {
		panic(err)
	}
	if err := registry.Register(NewXMLCompactTemplate()); err != nil {
		panic(err)
	}
	return registry
}
