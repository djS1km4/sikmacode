package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/djS1km4/sikmacode/internal/agent" // Importar el paquete agent
	"github.com/djS1km4/sikmacode/internal/config"
	"github.com/djS1km4/sikmacode/internal/llm"
	"github.com/djS1km4/sikmacode/internal/session"
	"github.com/google/generative-ai-go/genai"
)

// Define los mensajes para la comunicación asíncrona
type completionMsg struct {
	content string
}

type errorMsg struct {
	err error
}

func (e errorMsg) Error() string { return e.err.Error() }

// appModel representa el modelo principal de la aplicación TUI
type appModel struct {
	viewport     viewport.Model
	textarea     textarea.Model
	messages     []*genai.Content
	styles       Styles
	config       *config.Config
	apiKey       string
	sessionName  string
	keyMap       KeyMap
	isReady      bool
	agent        *agent.Agent // Añadir campo para el agente
	err          error
	width, height int
}

// NewAppModel inicializa un nuevo modelo de TUI
func NewAppModel(sessionName string) *appModel {
	styles := DefaultStyles()
	ta := textarea.New()
	ta.Placeholder = "Escribe tu mensaje aquí..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.Style{}

	vp := viewport.New(0, 0) // El tamaño se establecerá en el primer WindowSizeMsg
	ta.KeyMap.InsertNewline.SetEnabled(false)

	// Cargar configuración
	cfg, _ := config.LoadConfig() // Ignorar el error por ahora para simplificar
	var apiKey string
	if cfg != nil {
		apiKey, _ = cfg.GetActiveAPIKey()
	}

	// Cargar sesión si se especifica un nombre
	var messages []*genai.Content
	if sessionName != "" {
		loadedMessages, err := session.LoadSession(sessionName)
		if err == nil {
			messages = loadedMessages
		}
	} else {
		sessionName = fmt.Sprintf("session-%d", time.Now().Unix())
	}

	var agentInstance *agent.Agent
	if cfg != nil {
		agentInstance = agent.NewAgent(cfg.Agent)
	}

	return &appModel{
		textarea:    ta,
		viewport:    vp,
		styles:      styles,
		config:      cfg,
		apiKey:      apiKey,
		sessionName: sessionName,
		messages:    messages,
		keyMap:      DefaultKeyMap(),
		isReady:     false,
		agent:       agentInstance,
	}
}

// Init inicializa el modelo y devuelve comandos iniciales
func (m *appModel) Init() tea.Cmd {
	return textarea.Blink
}

// waitForCompletion es el comando que espera la respuesta del LLM.
func (m *appModel) waitForCompletion(userInput string) tea.Cmd {
	return func() tea.Msg {
		if m.config == nil {
			return errorMsg{fmt.Errorf("configuración no cargada")}
		}
		provider, ok := m.config.Providers[m.config.ActiveLLM]
		if !ok {
			return errorMsg{fmt.Errorf("proveedor de LLM activo no encontrado")}
		}
		modelName := provider.Model

		// Construir y pasar el System Prompt
		systemPrompt := m.agent.BuildSystemPrompt()

		response, err := llm.GenerateResponse(m.apiKey, modelName, systemPrompt, m.messages, userInput)
		if err != nil {
			return errorMsg{err}
		}
		return completionMsg{response}
	}
}

// Update maneja los mensajes entrantes y actualiza el estado de la aplicación.
func (m *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		cmds  []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calcular tamaños
		availableWidth := m.width - m.styles.AppStyle.GetHorizontalFrameSize()
		availableHeight := m.height - m.styles.AppStyle.GetVerticalFrameSize()
		m.textarea.SetWidth(availableWidth)
		viewportHeight := availableHeight - m.textarea.Height() - m.styles.InputField.GetVerticalFrameSize()
		m.viewport.Width = availableWidth
		m.viewport.Height = viewportHeight

		if !m.isReady {
			// Primera vez que se recibe WindowSizeMsg. Ahora que tenemos tamaño, 
			// podemos establecer el contenido inicial.
			if len(m.messages) > 0 {
				m.updateViewport()
				m.viewport.GotoBottom()
			} else {
				var initialContent string
				if m.config != nil {
					provider := m.config.Providers[m.config.ActiveLLM]
					initialContent = fmt.Sprintf("Bienvenido a Sikma Code!\nProveedor activo: %s (%s)", provider.Name, provider.Model)
					initialContent += fmt.Sprintf("\nNueva sesión: %s", m.sessionName)
				} else {
					initialContent = "Error al cargar la configuración."
				}
				m.viewport.SetContent(initialContent)
			}
			m.isReady = true
		}

		// Propagar el WindowSizeMsg a los componentes para que puedan procesarlo internamente
		m.textarea, tiCmd = m.textarea.Update(msg)
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, tiCmd, vpCmd)

		return m, tea.Batch(cmds...)

	case completionMsg:
		m.messages = append(m.messages, &genai.Content{
			Parts: []genai.Part{genai.Text(msg.content)},
			Role:  "model",
		})
		m.updateViewport()

	case errorMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keyMap.Save):
			err := session.SaveSession(m.sessionName, m.messages)
			if err != nil {
				// Manejar el error
			}
			return m, nil // Opcional: mostrar mensaje de guardado

		case msg.Type == tea.KeyEnter:
			if m.apiKey == "" {
				return m, nil
			}
			userInput := m.textarea.Value()
			m.messages = append(m.messages, &genai.Content{
				Parts: []genai.Part{genai.Text(userInput)},
				Role:  "user",
			})
			m.updateViewport()
			m.textarea.Reset()
			cmds = append(cmds, m.waitForCompletion(userInput))
		}
	}

	// Pasar el mensaje a los componentes anidados
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, tiCmd, vpCmd)

	return m, tea.Batch(cmds...)
}

// updateViewport actualiza el contenido del viewport con el historial de mensajes.
func (m *appModel) updateViewport() {
	var content strings.Builder
	for _, msg := range m.messages {
		role := "🤖"
		if msg.Role == "user" {
			role = "👤"
		}
		if len(msg.Parts) > 0 {
			if txt, ok := msg.Parts[0].(genai.Text); ok {
				content.WriteString(fmt.Sprintf("```markdown\n**%s**:\n%s\n```\n\n", role, string(txt))) // Añadido markdown para mejor renderizado
			}
		}
	}
	m.viewport.SetContent(content.String())
}

// View renderiza la interfaz completa de la aplicación.
func (m *appModel) View() string {
	if !m.isReady {
		return "Inicializando..."
	}
	// Si la ventana es demasiado pequeña, mostrar un mensaje
	minWidth, minHeight := 25, 10
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			m.styles.ErrorStyle.Render("Ventana demasiado pequeña!"),
		)
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %s\n\nPresiona Ctrl+C para salir.", m.err)
	}

	// Componer la vista principal
	appView := lipgloss.JoinVertical(
		lipgloss.Top,
		m.viewport.View(),
		m.styles.InputField.Render(m.textarea.View()),
	)

	return m.styles.AppStyle.Render(appView)
}