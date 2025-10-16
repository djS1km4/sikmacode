package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/djS1km4/sikmacode/internal/agent"
	"github.com/djS1km4/sikmacode/internal/config"
	"github.com/djS1km4/sikmacode/internal/llm"
	"github.com/djS1km4/sikmacode/internal/log"
	"github.com/djS1km4/sikmacode/internal/session"
	"github.com/djS1km4/sikmacode/internal/tools"
	"github.com/google/generative-ai-go/genai"
)

const logo = `░██████╗██╗██╗░░██╗███╗░░░███╗░█████╗░░█████╗░░█████╗░██████╗░███████╗
██╔════╝██║██║░██╔╝████╗░████║██╔══██╗██╔══██╗██╔══██╗██╔══██╗██╔════╝
╚█████╗░██║█████═╝░██╔████╔██║███████║██║░░╚═╝██║░░██║██║░░██║█████╗░░
░╚═══██╗██║██╔═██╗░██║╚██╔╝██║██╔══██║██║░░██╗██║░░██║██║░░██║██╔══╝░░
██████╔╝██║██║░╚██╗██║░╚═╝░██║██║░░██║╚█████╔╝╚█████╔╝██████╔╝███████╗
╚═════╝░╚═╝╚═╝░░╚═╝╚═╝░░░░░╚═╝╚═╝░░╚═╝░╚════╝░░╚════╝░╚═════╝░╚══════╝`

type completionMsg struct {
	content string
}

type errorMsg struct {
	err error
}

func (e errorMsg) Error() string { return e.err.Error() }

type appModel struct {
	viewport      viewport.Model
	textarea      textarea.Model
	messages      []*genai.Content
	styles        Styles
	config        *config.Config
	apiKey        string
	sessionName   string
	keyMap        KeyMap
		isReady          bool
		isSplashVisible  bool // Nuevo estado para controlar la visibilidad del logo grande
		agent            *agent.Agent
		err              error
		width, height     int
	}
	
	func NewAppModel(sessionName string) *appModel {
		styles := DefaultStyles()
		ta := textarea.New()
		ta.Placeholder = "ingresa tu pregunta para inciar : )"
		ta.Focus()
		ta.Prompt = ">"
	
		ta.ShowLineNumbers = false
	// Corrección para bug de letra fantasma y línea gris
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
		ta.SetHeight(3)
	
		vp := viewport.New(80, 20)
		ta.KeyMap.InsertNewline.SetEnabled(false)
	
		cfg, _ := config.LoadConfig()
		var apiKey string
		if cfg != nil {
			apiKey, _ = cfg.GetActiveAPIKey()
		}
	
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
			textarea:        ta,
			viewport:        vp,
			styles:          styles,
			config:          cfg,
			apiKey:          apiKey,
			sessionName:     sessionName,
			messages:        messages,
			keyMap:          DefaultKeyMap(),
			isReady:         false,
			isSplashVisible: true, // Inicia con el logo grande visible
			agent:           agentInstance,
		}
}

func (m *appModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *appModel) waitForCompletion(userInput string) tea.Cmd {
	return func() tea.Msg {
		var toolCalls []tools.ToolCall
		if err := json.Unmarshal([]byte(userInput), &toolCalls); err == nil && len(toolCalls) > 0 {
			return completionMsg{content: userInput}
		}

		if m.config == nil {
			return errorMsg{fmt.Errorf("configuración no cargada")}
		}
		provider, ok := m.config.Providers[m.config.ActiveLLM]
		if !ok {
			return errorMsg{fmt.Errorf("proveedor de LLM activo no encontrado")}
		}
		modelName := provider.Model

		systemPrompt := m.agent.BuildSystemPrompt()

		response, err := llm.GenerateResponse(m.apiKey, modelName, systemPrompt, m.messages, userInput)
		if err != nil {
			return errorMsg{err}
		}
		return completionMsg{response}
	}
}

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
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		dividerHeight := 1

		availableWidth := m.width - m.styles.AppStyle.GetHorizontalFrameSize()
		availableHeight := m.height - m.styles.AppStyle.GetVerticalFrameSize()

		m.textarea.SetWidth(availableWidth)
		m.viewport.Width = availableWidth
		m.viewport.Height = availableHeight - headerHeight - footerHeight - dividerHeight - m.textarea.Height()

		if !m.isReady {
			m.updateViewport()
			m.isReady = true
		}

	case completionMsg:
		var toolCalls []tools.ToolCall
		err := json.Unmarshal([]byte(msg.content), &toolCalls)
		log.Printf("Intento de parseo de tool call, err: %v, contenido: %s", err, msg.content)

		if err == nil && len(toolCalls) > 0 {
			var toolResultContent strings.Builder
			for _, call := range toolCalls {
				log.Printf("Ejecutando tool call: %+v", call)
				result, err := tools.Execute(call)
				if err != nil {
					toolResultContent.WriteString(fmt.Sprintf("Error al ejecutar la herramienta %s: %v\n", call.Name, err))
				} else {
					toolResultContent.WriteString(fmt.Sprintf("Resultado de %s: %s\n", call.Name, result))
				}
			}
			m.messages = append(m.messages, &genai.Content{
				Parts: []genai.Part{genai.Text(toolResultContent.String())},
				Role:  "model",
			})
		} else {
			m.messages = append(m.messages, &genai.Content{
				Parts: []genai.Part{genai.Text(msg.content)},
				Role:  "model",
			})
		}
		m.updateViewport()

	case errorMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keyMap.Save):
			session.SaveSession(m.sessionName, m.messages)
			return m, nil
		case msg.Type == tea.KeyEnter:
			userInput := m.textarea.Value()
			m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(userInput)}, Role: "user"})
			// Al enviar el primer mensaje, ocultamos el logo grande
			if m.isSplashVisible {
				m.isSplashVisible = false
			}
			m.updateViewport()
			m.textarea.Reset()
			cmds = append(cmds, m.waitForCompletion(userInput))
		}
	}

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, tiCmd, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *appModel) updateViewport() {
	renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())

	var content strings.Builder
	for i, msg := range m.messages {
		role := "🤖 Sikma"
		style := m.styles.AgentRole
		if msg.Role == "user" {
			role = "👤 Usuario"
			style = m.styles.UserRole
		}
		if len(msg.Parts) > 0 {
			if txt, ok := msg.Parts[0].(genai.Text); ok {
				formattedRole := style.Render(role)
				rendered, _ := renderer.Render(string(txt))
				content.WriteString(fmt.Sprintf("%s:\n%s", formattedRole, rendered))
				if i < len(m.messages)-1 {
					content.WriteString("\n\n---\n\n")
				}
			}
		}
	}
	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

const smallLogo = "≧◉◡◉≦"

func (m *appModel) headerView() string {
	if m.isSplashVisible {
		logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // Light Gray
		return lipgloss.JoinVertical(lipgloss.Center, logoStyle.Render(logo), "\nBienvenido a SikmaCode - una nueva experiencia de IA")
	}

	// Header pequeño con layout de dos columnas
	logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true) // Light Gray, Bold
	leftSide := logoStyle.Render(smallLogo) + "  " + m.sessionName

	timeStr := time.Now().Format("15:04:05")
	modelName := m.config.Providers[m.config.ActiveLLM].Model
	rightSide := fmt.Sprintf("%s | %s", modelName, timeStr)

	leftWidth := lipgloss.Width(leftSide)
	rightWidth := lipgloss.Width(rightSide)
	totalWidth := m.viewport.Width

	spacerWidth := totalWidth - leftWidth - rightWidth
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := strings.Repeat(" ", spacerWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftSide, spacer, rightSide)
}

func (m *appModel) footerView() string {
	return m.styles.FooterStyle.Render(fmt.Sprintf("  %s | %s", m.keyMap.Save.Help().Key, m.keyMap.Quit.Help().Key))
}

func (m *appModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	if !m.isReady {
		return "Inicializando..."
	}

	divider := m.styles.DividerStyle.Render(strings.Repeat("─", m.viewport.Width))

	// Construye el bloque de contenido principal
	mainContent := lipgloss.JoinVertical(
		lipgloss.Left,
		m.headerView(),
		m.viewport.View(),
		divider,
		m.textarea.View(),
		m.footerView(),
	)

	// Aplica el estilo del contenedor principal a todo el bloque
	return m.styles.AppStyle.Render(mainContent)
}
