package wizard

import "github.com/ScrawnDotDev/scrawn-cli/internal/ui"

type FieldType string

const (
	TypeSelect FieldType = "select"
	TypeInput  FieldType = "input"
)

type Field struct {
	Key          string
	Label        string
	Type         FieldType
	Options      []string
	DefaultValue string
	Validate     func(string) error
	AllowBlank   bool
	Description  string
}

func Input(key, label, description string, opts ...Option) Field {
	f := Field{
		Key:          key,
		Label:        label,
		Type:         TypeInput,
		Description: description,
	}
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

func Select(key, label, description string, options []string, opts ...Option) Field {
	f := Field{
		Key:         key,
		Label:       label,
		Type:        TypeSelect,
		Options:     options,
		Description: description,
	}
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

type Option func(*Field)

func WithDefault(value string) Option {
	return func(f *Field) {
		f.DefaultValue = value
	}
}

func WithValidator(fn func(string) error) Option {
	return func(f *Field) {
		f.Validate = fn
	}
}

func AllowBlank() Option {
	return func(f *Field) {
		f.AllowBlank = true
	}
}

func ToUI(fields []Field) []ui.WizardField {
	result := make([]ui.WizardField, len(fields))
	for i, f := range fields {
		result[i] = ui.WizardField{
			Key:          f.Key,
			Label:        f.Label,
			Type:         ui.FieldType(f.Type),
			Options:     f.Options,
			DefaultValue: f.DefaultValue,
			Validate:    f.Validate,
			AllowBlank:   f.AllowBlank,
			Description: f.Description,
		}
	}
	return result
}