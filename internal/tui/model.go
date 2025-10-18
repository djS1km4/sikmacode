package tui

import (
	"fmt"
	"os"
	"strings"
	"encoding/json"

	"github.com/djS1km4/sikmacode/internal/llm"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/glamour"
	"github.com/google/generative-ai-go/genai"
)

// completionMsg es el mensaje que se recibe cuando el LLM completa una respuesta.
type completionMsg struct{
	content string
}

// sessionEntry representa una entrada simple de sesión para persistencia en JSON.
type sessionEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// errorMsg es el mensaje para los errores que puedan ocurrir.
type errorMsg struct{ err error }

func (e errorMsg) Error() string { return e.err.Error() }

type Model struct {
	viewport    viewport.Model
	textarea    textarea.Model
	messages    []*genai.Content
	styles      Styles
	apiKey      string
	err         error
	renderer    *glamour.TermRenderer
}

func NewModel() Model {
	styles := DefaultStyles()
	ta := textarea.New()
	ta.Placeholder = "Escribe tu mensaje aquí..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 0 // Sin límite de caracteres
	ta.SetHeight(3)

	ta.FocusedStyle.CursorLine = lipgloss.Style{}

	vp := viewport.New(50, 5)
	apiKey := os.Getenv("GEMINI_API_KEY")

	if apiKey == "" {
		vp.SetContent(`Bienvenido a Sikma Code!
Error: La variable de entorno GEMINI_API_KEY no está configurada.`)
	} else {
		vp.SetContent(`Bienvenido a Sikma Code!
API Key detectada. Escribe un mensaje para comenzar.`)
	}

	ta.KeyMap.InsertNewline.SetEnabled(false)

	// Inicializa el renderer Markdown de Glamour a partir del entorno.
	r, _ := glamour.NewTermRenderer(glamour.WithEnvironmentConfig())

	m := Model{
		textarea: ta,
		viewport: vp,
		styles:   styles,
		apiKey:   apiKey,
		messages: make([]*genai.Content, 0),
		renderer: r,
	}

	// Carga sesión desde sessions/default.json si existe
	if data, err := os.ReadFile("sessions/default.json"); err == nil {
		var entries []sessionEntry
		if jsonErr := json.Unmarshal(data, &entries); jsonErr == nil {
			for _, e := range entries {
				m.messages = append(m.messages, &genai.Content{
					Role:  e.Role,
					Parts: []genai.Part{genai.Text(e.Content)},
				})
			}
			m.updateViewport()
		}
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// waitForCompletion es el comando que espera la respuesta del LLM.
func (m Model) waitForCompletion(userInput string) tea.Cmd {
	return func() tea.Msg {
		// El historial que se envía no incluye el mensaje actual del usuario.
		response, err := llm.GenerateResponse(m.apiKey, m.messages, userInput)
		if err != nil {
			return errorMsg{err}
		}
		return completionMsg{response}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		newWidth := int(float64(msg.Width) * 0.9)
		m.styles.InputField = m.styles.InputField.Width(newWidth)
		m.viewport.Width = newWidth
		m.textarea.SetWidth(newWidth)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(m.View()) + 1

	case completionMsg:
		m.messages = append(m.messages, &genai.Content{
		Parts: []genai.Part{genai.Text(msg.content)},
		Role:  "model",
	})
	m.updateViewport()
	m.saveSession()

	case errorMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.apiKey == "" {
				return m, nil
			}
			userInput := m.textarea.Value()
			// Añadir mensaje del usuario al historial.
			m.messages = append(m.messages, &genai.Content{
				Parts: []genai.Part{genai.Text(userInput)},
				Role:  "user",
			})
			m.updateViewport()
	m.saveSession()
	m.textarea.Reset()
	// Esperar la respuesta del LLM, pasando el input del usuario por separado.
	return m, m.waitForCompletion(userInput)
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

// updateViewport actualiza el contenido del viewport con el historial de mensajes.
func (m *Model) updateViewport() {
	var md strings.Builder
	for _, msg := range m.messages {
		role := "🤖"
		if msg.Role == "user" {
			role = "👤"
		}
		if len(msg.Parts) > 0 {
			if txt, ok := msg.Parts[0].(genai.Text); ok {
				// Construye contenido Markdown con énfasis en el rol
				md.WriteString(fmt.Sprintf("**%s**\n\n%s\n\n", role, string(txt)))
			}
		}
	}
	// Renderiza Markdown a salida con estilo usando Glamour
	if m.renderer != nil {
		if rendered, err := m.renderer.Render(md.String()); err == nil {
			m.viewport.SetContent(rendered)
		} else {
			m.viewport.SetContent(md.String())
		}
	} else {
		m.viewport.SetContent(md.String())
	}
	m.viewport.GotoBottom()
}

func (m *Model) saveSession() {
	entries := make([]sessionEntry, 0, len(m.messages))
	for _, c := range m.messages {
		var content string
		if len(c.Parts) > 0 {
			if txt, ok := c.Parts[0].(genai.Text); ok {
				content = string(txt)
			}
		}
		entries = append(entries, sessionEntry{Role: c.Role, Content: content})
	}
	if data, err := json.MarshalIndent(entries, "", "  "); err == nil {
		_ = os.WriteFile("sessions/default.json", data, 0644)
	}
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %s\n\nPresiona Ctrl+C para salir.", m.err)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		m.styles.InputField.Render(m.textarea.View()),
	)
}