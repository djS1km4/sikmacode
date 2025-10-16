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
	"github.com/charmbracelet/lipgloss"
	"github.com/djS1km4/sikmacode/internal/agent"
	"github.com/djS1km4/sikmacode/internal/config"
	"github.com/djS1km4/sikmacode/internal/llm"
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

const smallLogo = "≧ ◉ ◡ ◉ ≦"

type completionMsg struct{
	content string
}

type errorMsg struct {
	err error
}

func (e errorMsg) Error() string { return e.err.Error() }

type appModel struct {
	viewport         viewport.Model
	textarea         textarea.Model
	messages         []*genai.Content
	styles           Styles
	config           *config.Config
	apiKey           string
	sessionName      string
	keyMap           KeyMap
	isReady          bool
	isSplashVisible  bool
	isConfirming     bool
	confirmPrompt    string
	confirmationType string
	pendingCall      *tools.ToolCall
	allowedTools     map[string]bool
	agent            *agent.Agent
	err              error
	width, height     int
}

func NewAppModel(sessionName string) *appModel {
	styles := DefaultStyles()
	ta := textarea.New()
	ta.Placeholder = "Escribe tu mensaje aquí..."
	ta.Focus()
	ta.Prompt = "> "
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

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
		isSplashVisible: true,
		allowedTools:    make(map[string]bool),
		agent:           agentInstance,
	}
}

func (m *appModel) Init() tea.Cmd {
	return textarea.Blink
}

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

		systemPrompt := m.agent.BuildSystemPrompt()

		response, err := llm.GenerateResponse(m.apiKey, modelName, systemPrompt, m.messages, userInput)
		if err != nil {
			return errorMsg{err}
		}
		return completionMsg{response}
	}
}

// executeAndLog es una función helper que ejecuta una herramienta y loguea el resultado.
func (m *appModel) executeAndLog(call tools.ToolCall) {
	result, err := tools.Execute(call)
	var resultMsg string
	if err != nil {
		resultMsg = fmt.Sprintf("Error al ejecutar la herramienta %s: %v", call.Name, err)
	} else {
		resultMsg = result
	}
	m.messages = append(m.messages, &genai.Content{
		Parts: []genai.Part{genai.Text(resultMsg)},
		Role:  "model",
	})
	m.updateViewport()
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
		
		mainContentHeight := m.height - headerHeight - footerHeight
		availableWidth := m.width - m.styles.AppStyle.GetHorizontalFrameSize()
		availableHeight := mainContentHeight - m.styles.AppStyle.GetVerticalFrameSize()

		dividerHeight := 1
		m.textarea.SetWidth(availableWidth)
		m.viewport.Width = availableWidth
		m.viewport.Height = availableHeight - dividerHeight - m.textarea.Height()

		if !m.isReady {
			m.updateViewport()
			m.isReady = true
		}

	case completionMsg:
		content := msg.content
		jsonStr := content

		if start := strings.Index(content, "```json"); start != -1 {
			if end := strings.LastIndex(content, "```"); end > start {
				jsonStr = content[start+len("```json") : end]
				jsonStr = strings.TrimSpace(jsonStr)
			}
		}

		var rawToolCalls []map[string]interface{}
		err := json.Unmarshal([]byte(jsonStr), &rawToolCalls)
		if err != nil {
			var singleRawCall map[string]interface{}
			err2 := json.Unmarshal([]byte(jsonStr), &singleRawCall)
			if err2 == nil {
				rawToolCalls = []map[string]interface{}{singleRawCall}
				err = nil
			}
		}

		if err == nil && len(rawToolCalls) > 0 {
			for _, rawCall := range rawToolCalls {
				call := tools.ToolCall{}
				if name, ok := rawCall["tool"].(string); ok {
					call.Name = name
				} else if name, ok := rawCall["tool_name"].(string); ok {
					call.Name = name
				} else if name, ok := rawCall["tool_code"].(string); ok {
					call.Name = name
				}

				if args, ok := rawCall["kwargs"].(map[string]interface{}); ok {
					call.Arguments = args
				} else if args, ok := rawCall["parameters"].(map[string]interface{}); ok {
					call.Arguments = args
				} else {
					call.Arguments = rawCall
				}

				// La TUI decide si se necesita confirmación
				switch call.Name {
				case "file:write", "file:read":
					if m.allowedTools[call.Name] {
						m.executeAndLog(call)
					} else {
						m.pendingCall = &call
						m.isConfirming = true
						m.confirmPrompt = fmt.Sprintf("El agente quiere usar `%s`. Proceder?", call.Name)
						m.confirmationType = "S/N/A"
						return m, nil
					}
				case "bash:execute":
					m.pendingCall = &call
					m.isConfirming = true
					m.confirmPrompt = fmt.Sprintf("El agente quiere ejecutar: `%s`. Proceder?", call.Arguments["command"])
					m.confirmationType = "S/N"
						return m, nil
				default:
					m.executeAndLog(call)
				}
			}
		} else {
			m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(msg.content)}, Role: "model"})
			m.updateViewport()
		}

	case errorMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		if m.isConfirming {
			switch strings.ToLower(msg.String()) {
			case "s", "y":
				m.isConfirming = false
				if m.pendingCall != nil {
					m.executeAndLog(*m.pendingCall)
					m.pendingCall = nil
				}
			case "n":
				m.isConfirming = false
				m.pendingCall = nil
				m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Acción cancelada por el usuario.")}, Role: "model"})
				m.updateViewport()
			case "a":
				if m.confirmationType == "S/N/A" {
					m.isConfirming = false
					if m.pendingCall != nil {
						m.allowedTools[m.pendingCall.Name] = true
						m.executeAndLog(*m.pendingCall)
						m.pendingCall = nil
					}
				}
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keyMap.Save):
			session.SaveSession(m.sessionName, m.messages)
			return m, nil
		case msg.Type == tea.KeyEnter:
			userInput := m.textarea.Value()
			m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(userInput)}, Role: "user"})
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
	var content strings.Builder
	for i, msg := range m.messages {
		role := "🤖 Agente"
		style := m.styles.AgentRole
		if msg.Role == "user" {
			role = "👤 Usuario"
			style = m.styles.UserRole
		}
		if len(msg.Parts) > 0 {
			if txt, ok := msg.Parts[0].(genai.Text); ok {
				formattedRole := style.Render(role)
				wrappedText := lipgloss.NewStyle().Width(m.viewport.Width).Render(string(txt))
				content.WriteString(fmt.Sprintf("%s:\n%s", formattedRole, wrappedText))
				if i < len(m.messages)-1 {
					content.WriteString("\n\n---\n\n")
				}
			}
		}
	}
	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

func (m *appModel) headerView() string {
	if m.isSplashVisible {
		logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		return lipgloss.JoinVertical(lipgloss.Center, logoStyle.Render(logo), "\nBienvenido a SikmaCode - una nueva experiencia de IA")
	}

	logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true)
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

	if m.isConfirming {
		var options string
		if m.confirmationType == "S/N/A" {
			options = "(s/n/a)"
		} else {
			options = "(s/n)"
		}
		dialogBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("228")).
			Padding(1, 2).
			Render(m.confirmPrompt + "\n\n" + options)

		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			dialogBox,
		)
	}

	divider := m.styles.DividerStyle.Render(strings.Repeat("─", m.viewport.Width))

	// Contenido que va dentro del borde
	innerContent := lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		divider,
		m.textarea.View(),
	)
	borderedContent := m.styles.AppStyle.Render(innerContent)

	// Ensamblaje final
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.headerView(),
		borderedContent,
		m.footerView(),
	)
}
