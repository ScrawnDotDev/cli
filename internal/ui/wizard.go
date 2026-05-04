package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var ErrPromptInterrupted = errors.New("prompt interrupted")

type FieldType string

const (
	FieldSelect FieldType = "select"
	FieldInput  FieldType = "input"
)

type WizardField struct {
	Key          string
	Label        string
	Type         FieldType
	Options      []string
	DefaultValue string
	Validate     func(string) error
	AllowBlank   bool
	Description  string
}

type wizardModel struct {
	title       string
	description string
	fields      []WizardField
	index       int
	values      map[string]string
	selected    int
	input       textinput.Model
	err         error
	interrupted bool
	done        bool
	width       int
}

func RunWizard(title string, description string, fields []WizardField) (map[string]string, error) {
	model := newWizardModel(title, description, fields)
	program := tea.NewProgram(model)
	result, err := program.Run()
	if err != nil {
		return nil, err
	}

	finalModel, ok := result.(wizardModel)
	if !ok {
		return nil, errors.New("unexpected wizard result")
	}
	if finalModel.interrupted {
		return nil, ErrPromptInterrupted
	}
	return finalModel.values, nil
}

func newWizardModel(title string, description string, fields []WizardField) wizardModel {
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 4096
	input.Width = 72
	input.Cursor.Style = valueStyle

	model := wizardModel{
		title:       title,
		description: description,
		fields:      fields,
		values:      map[string]string{},
		input:       input,
	}
	model.activateCurrentField()
	return model
}

func (m wizardModel) Init() tea.Cmd {
	if m.currentField().Type == FieldInput {
		return textinput.Blink
	}
	return nil
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.interrupted = true
			m.done = true
			return m, tea.Quit
		}
	}

	if m.done {
		return m, tea.Quit
	}

	field := m.currentField()
	if field.Type == FieldSelect {
		return m.updateSelect(msg)
	}
	return m.updateInput(msg)
}

func (m wizardModel) View() string {
	if m.done {
		return ""
	}

	field := m.currentField()
	progress := fmt.Sprintf("[%d/%d]", m.index+1, len(m.fields))
	body := []string{
		sectionStyle.Render("Scrawn CLI"),
		mutedStyle.Render("> " + m.title),
		subtleRule(),
		progress + " " + field.Label + ":",
	}

	if strings.TrimSpace(field.Description) != "" {
		body = append(body, mutedStyle.Render(field.Description))
	}

	if field.Type == FieldSelect {
		body = append(body, "")
		for index, option := range field.Options {
			prefix := "  "
			style := mutedStyle
			if index == m.selected {
				prefix = stepStyle.Render("> ")
				style = valueStyle
			}
			body = append(body, prefix+style.Render(option))
		}
		body = append(body, "", mutedStyle.Render("Use up/down and press Enter."))
	} else {
		body = append(body, "", valueStyle.Render(m.input.View()))
		if m.err != nil {
			body = append(body, failureStyle.Render(m.err.Error()))
		}
		body = append(body, mutedStyle.Render("Press Enter to continue."))
	}

return strings.Join(body, "\n")
}

func (m wizardModel) updateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.currentField().Options)-1 {
				m.selected++
			}
		case "enter":
			m.values[m.currentField().Key] = m.currentField().Options[m.selected]
			return m.advance()
		}
	}
	return m, nil
}

func (m wizardModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		value := strings.TrimSpace(m.input.Value())
		field := m.currentField()

		if value == "" {
			if field.DefaultValue != "" {
				value = field.DefaultValue
			} else if !field.AllowBlank {
				m.err = fmt.Errorf("this field is required")
				return m, textinput.Blink
			}
		}

		if field.Validate != nil && value != "" {
			if err := field.Validate(value); err != nil {
				m.err = err
				return m, textinput.Blink
			}
		}
		m.err = nil
		m.values[field.Key] = value
		return m.advance()
	}

	return m, cmd
}

func (m wizardModel) advance() (tea.Model, tea.Cmd) {
	if m.index >= len(m.fields)-1 {
		m.done = true
		return m, tea.Quit
	}

	m.index++
	m.err = nil
	m.activateCurrentField()
	if m.currentField().Type == FieldInput {
		return m, textinput.Blink
	}
	return m, nil
}

func (m *wizardModel) activateCurrentField() {
	field := m.currentField()
	if field.Type == FieldSelect {
		m.selected = 0
		for index, option := range field.Options {
			if option == field.DefaultValue {
				m.selected = index
				break
			}
		}
		return
	}

	m.input = textinput.New()
	m.input.Prompt = ""
	m.input.CharLimit = 4096
	m.input.Width = 72
	m.input.Cursor.Style = valueStyle
	m.input.SetValue("")
	m.input.CursorEnd()
	m.input.Focus()
	if field.DefaultValue != "" {
		m.input.Placeholder = field.DefaultValue
	}
}

func (m wizardModel) currentField() WizardField {
	return m.fields[m.index]
}
