package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"regexp"
	"math/rand"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/glamour"
	"github.com/djS1km4/sikmacode/internal/agent"
	"github.com/djS1km4/sikmacode/internal/config"
	"github.com/djS1km4/sikmacode/internal/llm"
	"github.com/djS1km4/sikmacode/internal/session"
	"github.com/djS1km4/sikmacode/internal/tools"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
)

const logo = `░██████╗██╗██╗░░██╗███╗░░░███╗░█████╗░░█████╗░░█████╗░██████╗░███████╗
██╔════╝██║██║░██╔╝████╗░████║██╔══██╗██╔══██╗██╔══██╗██╔══██╗██╔════╝
╚█████╗░██║█████═╝░██╔████╔██║███████║██║░░╚═╝██║░░██║██║░░██║█████╗░░
░╚═══██╗██║██╔═██╗░██║╚██╔╝██║██╔══██║██║░░██╗██║░░██║██║░░██║██╔══╝░░
██████╔╝██║██║░╚██╗██║░╚═╝░██║██║░░██║╚█████╔╝╚█████╔╝██████╔╝███████╗
╚═════╝░╚═╝╚═╝░░╚═╝╚═╝░░░░░╚═╝╚═╝░░╚═╝░╚════╝░░╚════╝░╚══════╝`

const smallLogo = "≧ ◉ ◡ ◉ ≦"

type completionMsg struct{
	content string
}

type errorMsg struct {
	err error
}

// Mensajes para streaming
type streamChunkMsg struct{ chunk string }
type streamDoneMsg struct{}

// Nuevo: mensaje de tick para barra de carga
type loadingTickMsg struct{ frame string }

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
	// Streaming
	isStreaming      bool
	streamBuffer     strings.Builder
	streamClient     *genai.Client
	streamIter       *genai.GenerateContentResponseIterator
	// Barra de carga
	loadingBar       string
	// Ayuda
	showHelp         bool
	// Seguridad UX
	awaitingDoubleConfirm bool
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

// Comando para iniciar streaming
func (m *appModel) startStreamCmd(userInput string) tea.Cmd {
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
		client, iter, err := llm.StartStream(m.apiKey, modelName, systemPrompt, m.messages, userInput)
		if err != nil {
			return errorMsg{err}
		}
		m.streamClient = client
		m.streamIter = iter
		m.isStreaming = true
		m.streamBuffer.Reset()
		// Inicializar barra de carga
		m.loadingBar = randomBar(15)
		// Leer primer chunk
		return m.readNextStreamChunkMsg()
	}
}

func (m *appModel) readNextStreamChunkMsg() tea.Msg {
	if m.streamIter == nil {
		return streamDoneMsg{}
	}
	resp, err := m.streamIter.Next()
	if err == iterator.Done {
		return streamDoneMsg{}
	}
	if err != nil {
		return errorMsg{err}
	}
	var chunk strings.Builder
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, p := range resp.Candidates[0].Content.Parts {
			if t, ok := p.(genai.Text); ok {
				chunk.WriteString(string(t))
			}
		}
	}
	return streamChunkMsg{chunk: chunk.String()}
}

// nextStreamChunkCmd moved to bottom of file to group with tick utilities

// sanitizeAgentText elimina bloques JSON de tool-calls del texto del agente.
func sanitizeAgentText(s string) string {
	// Remover bloques con fences ```json ... ```
	reFenceArray := regexp.MustCompile("(?s)```json\\s*\\[.*?\\]\\s*```")
	s = reFenceArray.ReplaceAllString(s, "")
	reFenceObject := regexp.MustCompile("(?s)```json\\s*\\{[\\s\\S]*?\\}" )
	s = reFenceObject.ReplaceAllString(s, "")
	// Remover JSON plano que contenga la clave \"tool\"
	rePlainArray := regexp.MustCompile(`(?s)\[\s*\{[\s\S]*?"tool"[\s\S]*?\}\s*\]`)
	s = rePlainArray.ReplaceAllString(s, "")
	rePlainObject := regexp.MustCompile(`(?s)\{[\s\S]*?"tool"[\s\S]*?\}`)
	s = rePlainObject.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
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
		// Clamp de seguridad para evitar alturas negativas
		if m.viewport.Height < 3 {
			m.viewport.Height = 3
		}

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
					// Marcar si requiere doble confirmación
					if cmd, ok := call.Arguments["command"].(string); ok {
						m.awaitingDoubleConfirm = tools.IsCriticalCommand(cmd)
					} else {
						m.awaitingDoubleConfirm = false
					}
					return m, nil
				case "ask_user_confirmation":
					m.pendingCall = &call
					m.isConfirming = true
					prompt := "¿Confirmar acción del agente?"
					if p, ok := call.Arguments["prompt"].(string); ok && p != "" {
						prompt = p
					}
					m.confirmPrompt = prompt
					m.confirmationType = "S/N"
					return m, nil
				default:
					m.executeAndLog(call)
				}
			}
		} else {
			clean := sanitizeAgentText(msg.content)
			if strings.TrimSpace(clean) != "" {
				m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(clean)}, Role: "model"})
			}
			m.updateViewport()
		}

	case streamChunkMsg:
		if msg.chunk != "" {
			m.streamBuffer.WriteString(msg.chunk)
			m.updateViewport()
		}
		return m, tea.Batch(m.nextStreamChunkCmd(), m.loadingTickCmd())

	case streamDoneMsg:
		if m.streamClient != nil {
			m.streamClient.Close()
		}
		finalText := m.streamBuffer.String()
		if finalText != "" {
			// Intentar parsear tool-calls como en completionMsg
			jsonStr := finalText
			if start := strings.Index(finalText, "```json"); start != -1 {
				if end := strings.LastIndex(finalText, "```"); end > start {
					jsonStr = finalText[start+len("```json") : end]
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
					switch call.Name {
					case "file:write", "file:read":
						if m.allowedTools[call.Name] {
							m.executeAndLog(call)
						} else {
							m.pendingCall = &call
							m.isConfirming = true
							m.confirmPrompt = fmt.Sprintf("El agente quiere usar `%s`. Proceder?", call.Name)
							m.confirmationType = "S/N/A"
							m.updateViewport()
							return m, nil
						}
					case "bash:execute":
						m.pendingCall = &call
						m.isConfirming = true
						m.confirmPrompt = fmt.Sprintf("El agente quiere ejecutar: `%s`. Proceder?", call.Arguments["command"])
						m.confirmationType = "S/N"
						// Marcar si requiere doble confirmación
						if cmd, ok := call.Arguments["command"].(string); ok {
							m.awaitingDoubleConfirm = tools.IsCriticalCommand(cmd)
						} else {
							m.awaitingDoubleConfirm = false
						}
						m.updateViewport()
						return m, nil
					case "ask_user_confirmation":
						m.pendingCall = &call
						m.isConfirming = true
						prompt := "¿Confirmar acción del agente?"
						if p, ok := call.Arguments["prompt"].(string); ok && p != "" {
							prompt = p
						}
						m.confirmPrompt = prompt
						m.confirmationType = "S/N"
						m.updateViewport()
						return m, nil
					default:
						m.executeAndLog(call)
					}
				}
			} else {
				clean := sanitizeAgentText(finalText)
				if strings.TrimSpace(clean) != "" {
					m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(clean)}, Role: "model"})
				}
			}
		}
		m.isStreaming = false
		m.streamIter = nil
		m.loadingBar = ""
		m.updateViewport()
		return m, nil

	case loadingTickMsg:
		m.loadingBar = msg.frame
		if m.isStreaming {
			m.updateViewport()
			return m, m.loadingTickCmd()
		}
		return m, nil

	case errorMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		if m.showHelp {
			if key.Matches(msg, m.keyMap.Help) {
				m.showHelp = false
				return m, nil
			}
			if key.Matches(msg, m.keyMap.Quit) {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.isConfirming {
			switch strings.ToLower(msg.String()) {
			case "s", "y":
				// Manejar doble confirmación para bash:execute
				if m.pendingCall != nil && m.pendingCall.Name == "bash:execute" && m.awaitingDoubleConfirm {
					m.confirmPrompt = fmt.Sprintf("Comando crítico detectado: `%s`. Confirmar otra vez?", m.pendingCall.Arguments["command"])
					m.awaitingDoubleConfirm = false
					m.isConfirming = true
					m.updateViewport()
					return m, nil
				}
				m.isConfirming = false
				if m.pendingCall != nil {
					if m.pendingCall.Name == "ask_user_confirmation" {
						m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("CONFIRMATION: yes")}, Role: "model"})
						m.updateViewport()
						m.pendingCall = nil
					} else {
						m.executeAndLog(*m.pendingCall)
						m.pendingCall = nil
					}
				}
			case "n":
				m.isConfirming = false
				if m.pendingCall != nil && m.pendingCall.Name == "ask_user_confirmation" {
					m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("CONFIRMATION: no")}, Role: "model"})
					m.updateViewport()
					m.pendingCall = nil
				} else {
					m.pendingCall = nil
					m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Acción cancelada por el usuario.")}, Role: "model"})
					m.updateViewport()
				}
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
			m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text("Sesión guardada.")}, Role: "model"})
			m.updateViewport()
			return m, nil
		case key.Matches(msg, m.keyMap.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case msg.Type == tea.KeyEnter:
			userInput := m.textarea.Value()
			trimmed := strings.TrimSpace(userInput)
			if strings.HasPrefix(trimmed, "/help") {
				m.showHelp = !m.showHelp
				m.textarea.Reset()
				return m, nil
			}
			m.messages = append(m.messages, &genai.Content{Parts: []genai.Part{genai.Text(userInput)}, Role: "user"})
			if m.isSplashVisible {
				m.isSplashVisible = false
			}
			m.updateViewport()
			m.textarea.Reset()
			return m, tea.Batch(m.startStreamCmd(userInput), m.loadingTickCmd())
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

				textToRender := string(txt)
				if msg.Role != "user" {
					textToRender = sanitizeAgentText(textToRender)
					if strings.TrimSpace(textToRender) == "" {
						// Nada que mostrar tras sanitizar
						continue
					}
				}

				renderer, _ := glamour.NewTermRenderer(
					glamour.WithEnvironmentConfig(),
					glamour.WithAutoStyle(),
					glamour.WithWordWrap(m.viewport.Width),
				)
				rendered, err := renderer.Render(textToRender)
				if err != nil {
					rendered = lipgloss.NewStyle().Width(m.viewport.Width).Render(textToRender)
				}

				content.WriteString(fmt.Sprintf("%s:\n%s", formattedRole, rendered))
				if i < len(m.messages)-1 {
					content.WriteString("\n\n---\n\n")
				}
			}
		}
	}
	// Añadir buffer de streaming en vivo
	printedStreaming := false
	if m.isStreaming && m.streamBuffer.Len() > 0 {
		formattedRole := m.styles.AgentRole.Render("🤖 Agente")
		raw := m.streamBuffer.String()
		clean := sanitizeAgentText(raw)
		if strings.TrimSpace(clean) != "" {
			renderer, _ := glamour.NewTermRenderer(
				glamour.WithEnvironmentConfig(),
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.viewport.Width),
			)
			rendered, err := renderer.Render(clean)
			if err != nil {
				rendered = lipgloss.NewStyle().Width(m.viewport.Width).Render(clean)
			}
			if content.Len() > 0 {
				content.WriteString("\n\n---\n\n")
			}
			content.WriteString(fmt.Sprintf("%s:\n%s", formattedRole, rendered))
			printedStreaming = true
		}
	}
	// Añadir barra de carga dinámica cuando está en streaming
	if m.isStreaming {
		bar := m.styles.LoadingStyle.Render(m.loadingBar)
		if printedStreaming {
			content.WriteString("\n" + bar)
		} else {
			// No se imprimió contenido del agente aún; mostrar etiqueta y barra
			if content.Len() > 0 {
				content.WriteString("\n\n---\n\n")
			}
			formattedRole := m.styles.AgentRole.Render("🤖 Agente")
			content.WriteString(fmt.Sprintf("%s:\n%s", formattedRole, bar))
		}
	}
	m.viewport.SetContent(content.String())
	// Evitar pánico si la altura es muy pequeña
	if m.viewport.Height > 0 {
		m.viewport.GotoBottom()
	}
}

func (m *appModel) headerView() string {
	if m.isSplashVisible {
		logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		return lipgloss.JoinVertical(lipgloss.Center, logoStyle.Render(logo), "\nBienvenido a SikmaCode - una nueva experiencia de IA")
	}

	logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true)
	leftSide := logoStyle.Render(smallLogo) + "  " + m.sessionName

	timeStr := time.Now().Format("15:04:05")
	provider := m.config.Providers[m.config.ActiveLLM]
	modelName := provider.Model
	providerName := provider.Name
	status := "Idle"
	if m.isStreaming {
		status = "Streaming"
	}
	rightSide := fmt.Sprintf("%s (%s) | %s | %s", providerName, modelName, timeStr, status)

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
	help := "[Ctrl+S guardar] [Ctrl+C/Esc salir] [Ayuda: F1]"
	return m.styles.FooterStyle.Render(help)
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

	if m.showHelp {
		content := strings.Join([]string{
			"Ayuda rápida",
			"",
			"- Enter: enviar mensaje",
			"- Ctrl+S: guardar sesión",
			"- Ctrl+C / Esc: salir",
			"- F1: mostrar/ocultar esta ayuda",
			"- /help: alterna ayuda desde el prompt",
			"",
			"Confirmaciones:",
			"- s/y: sí",
			"- n: no",
			"- a: siempre permitir (solo herramientas seguras)",
			"- Nota: comandos críticos requieren doble confirmación",
			"",
			"Estado:",
			"- Header muestra proveedor/modelo, hora y estado (Idle/Streaming)",
		}, "\n")
		dialogBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Render(content)
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


func (m *appModel) nextStreamChunkCmd() tea.Cmd {
	return func() tea.Msg { return m.readNextStreamChunkMsg() }
}

// Nuevo: comando tick para actualizar barra de carga
func (m *appModel) loadingTickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return loadingTickMsg{frame: randomBar(15)}
	})
}

// Genera una barra de 15 caracteres con símbolos aleatorios
func randomBar(n int) string {
	chars := []rune{'█','▓','▒','░','─','━','╌','╍','▌','▐','▍','▎','▏'}
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}
