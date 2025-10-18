package tui

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/djS1km4/sikmacode/internal/agent"
	"github.com/djS1km4/sikmacode/internal/config"
	"github.com/djS1km4/sikmacode/internal/llm"
	"github.com/djS1km4/sikmacode/internal/session"
	"github.com/djS1km4/sikmacode/internal/tools"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
)

const logo = `
░██████╗██╗██╗░░██╗███╗░░░███╗░█████╗░░█████╗░░█████╗░██████╗░███████╗
██╔════╝██║██║░██╔╝████╗░████║██╔══██╗██╔══██╗██╔══██╗██╔══██╗██╔════╝
╚█████╗░██║█████═╝░██╔████╔██║███████║██║░░╚═╝██║░░██║██║░░██║█████╗░░
░╚═══██╗██║██╔═██╗░██║╚██╔╝██║██╔══██║██║░░██╗██║░░██║██║░░██║██╔══╝░░
██████╔╝██║██║░╚██╗██║░╚═╝░██║██║░░██║╚█████╔╝╚█████╔╝██████╔╝███████╗
╚═════╝░╚═╝╚═╝░░╚═╝╚═╝░░░░░╚═╝╚═╝░░╚═╝░╚════╝░░╚════╝░╚══════╝
`

const smallLogo = "≧◉◡◉≦"

type completionMsg struct {
	content string
}

type errorMsg struct {
	err error
}

// Mensajes para streaming
type streamChunkMsg struct{ chunk string }
type streamDoneMsg struct{}

type loadingTickMsg struct{ frame string }

// Nuevo: mensaje de frase de pensamiento
type thinkingTickMsg struct{ phrase string }

// Nuevo: animación de chips
type chipAnimTickMsg struct{}
// Nuevo: parpadeo de kaomoji
type kaomojiBlinkMsg struct{}
type kaomojiResetMsg struct{}

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
	width, height    int
	// Streaming
	isStreaming  bool
	streamBuffer strings.Builder
	streamClient *genai.Client
	streamIter   *genai.GenerateContentResponseIterator
	// Barra de carga
	loadingBar string
	// Precarga (antes de responder)
	isPreloading bool
	preloadText  string
	// Texto motivacional en cabecera durante pensamiento
	thinkingPhrase string
	streamStart    time.Time
	// Metadatos última respuesta para chips
	lastResponseDuration    time.Duration
	lastResponseTokenApprox int
	lastResponseProvider    string
	lastResponseModel       string
	// Ayuda
	showHelp bool
	// Seguridad UX
	awaitingDoubleConfirm bool
	// Spinner visual
	spin spinner.Model
	// Tema/acento actual
	themeAccentIdx int
	// Progreso animado (Harmonica via Bubbles)
	prog progress.Model
	// Nuevo: offset de animación para chips
	chipAnimOffset int
	// Nuevo: kaomoji actual (para parpadeo sutil)
	currentKao string
	kaoDoDouble bool
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

	// Inicializar spinner visual
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))
	// Progreso con gradiente y resorte
	pr := progress.New(
		progress.WithDefaultScaledGradient(),
		progress.WithSpringOptions(6.0, 0.5),
	)

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
	// Sembrar aleatoriedad para barras y precarga
	rand.Seed(time.Now().UnixNano())

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
		spin:            sp,
		themeAccentIdx:  0,
		prog:            pr,
		chipAnimOffset:  0,
		currentKao:      "≧◉◡◉≦",
		kaoDoDouble:     false,
	}
}

func (m *appModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.kaomojiIdleTickCmd())
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
		m.loadingBar = randomBar(15)
		m.streamStart = time.Now()
		// Frase inicial mientras piensa
		m.thinkingPhrase = randomThinkingPhrase()
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

func (m *appModel) preloadTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return preloadTickMsg{text: randomAlphaNum(16)}
	})
}

// nextStreamChunkCmd moved to bottom of file to group with tick utilities

// sanitizeAgentText elimina bloques JSON de tool-calls del texto del agente.
func sanitizeAgentText(s string) string {
	// Remover bloques con fences ```json ... ```
	reFenceArray := regexp.MustCompile("(?s)```json\\s*\\[.*?\\]\\s*```")
	s = reFenceArray.ReplaceAllString(s, "")
	reFenceObject := regexp.MustCompile("(?s)```json\\s*\\{[\\s\\S]*?\\}")
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
		// Ajustar ancho de barra de progreso en header (compacta)
		pw := availableWidth / 8
		if pw < 10 {
			pw = 10
		}
		if pw > 20 {
			pw = 20
		}
		m.prog.Width = pw
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
			m.isPreloading = false
			m.preloadText = ""
			m.updateViewport()
		}
		return m, tea.Batch(m.nextStreamChunkCmd(), m.loadingTickCmd(), m.spin.Tick)

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
		// Registrar metadatos para chips
		m.lastResponseDuration = time.Since(m.streamStart)
		if m.config != nil {
			if p, ok := m.config.Providers[m.config.ActiveLLM]; ok {
				m.lastResponseProvider = p.Name
				m.lastResponseModel = p.Model
			}
		}
		// Aproximación simple de tokens
		runes := []rune(finalText)
		m.lastResponseTokenApprox = len(runes) / 4
		m.isStreaming = false
		m.streamIter = nil
		m.loadingBar = ""
		m.isPreloading = false
		m.preloadText = ""
		m.thinkingPhrase = ""
		// Iniciar animación de chips al finalizar streaming
		m.chipAnimOffset = 10
		// Animar progreso al 100%
		pcmd := m.prog.SetPercent(1.0)
		m.updateViewport()
		return m, tea.Batch(m.chipAnimTickCmd(), pcmd)

	case thinkingTickMsg:
		// Actualizar frase solo durante el pensamiento (precarga)
		if m.isPreloading {
			m.thinkingPhrase = msg.phrase
			m.updateViewport()
			return m, m.thinkingTickCmd()
		}
		return m, nil

	case preloadTickMsg:
		if m.isPreloading {
			m.preloadText = msg.text
			m.updateViewport()
			return m, m.preloadTickCmd()
		}
		return m, nil

	case loadingTickMsg:
		m.loadingBar = msg.frame
		if m.isStreaming {
			var pcmd tea.Cmd
			if m.prog.Percent() >= 0.95 {
				pcmd = m.prog.SetPercent(0)
			} else {
				pcmd = m.prog.IncrPercent(0.06)
			}
			m.updateViewport()
			return m, tea.Batch(m.loadingTickCmd(), pcmd, m.spin.Tick)
		}
		return m, nil

	case chipAnimTickMsg:
		if m.chipAnimOffset > 0 {
			m.chipAnimOffset--
			m.updateViewport()
			return m, m.chipAnimTickCmd()
		}
		return m, nil

	// Parpadeo sutil del kaomoji
	case kaomojiBlinkMsg:
		m.currentKao = m.buildKao(false)
		return m, m.kaomojiResetTickCmd()
	case kaomojiResetMsg:
		m.currentKao = m.buildKao(true)
		// Si está marcado un doble parpadeo, disparamos un segundo parpadeo pronto.
		if m.kaoDoDouble {
			m.kaoDoDouble = false
			return m, m.kaomojiSecondBlinkTickCmd()
		}
		return m, m.kaomojiIdleTickCmd()

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
		case key.Matches(msg, m.keyMap.Theme):
			m.cycleAccentTheme()
			m.updateViewport()
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
			m.isPreloading = true
			m.preloadText = randomAlphaNum(16)
			// Programar spinner y frase desde el inicio del pensamiento
			return m, tea.Batch(m.startStreamCmd(userInput), m.preloadTickCmd(), m.loadingTickCmd(), m.spin.Tick, m.thinkingTickCmd(), m.prog.SetPercent(0.0))
		}
	}

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	var spCmd tea.Cmd
	m.spin, spCmd = m.spin.Update(msg)
	// Ajuste: progress.Update devuelve tea.Model, hacemos type assert
	var prCmd tea.Cmd
	var progModel tea.Model
	progModel, prCmd = m.prog.Update(msg)
	if pm, ok := progModel.(progress.Model); ok {
		m.prog = pm
	}
	cmds = append(cmds, tiCmd, vpCmd, spCmd, prCmd)

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
	// Mostrar precarga alfanumérica (antes de responder)
	if m.isPreloading {
		if content.Len() > 0 {
			content.WriteString("\n\n---\n\n")
		}
		phrase := m.styles.ThinkingStyle.Render(m.thinkingPhrase)
		line := fmt.Sprintf("%s %s", m.spin.View(), phrase)
		content.WriteString(line + "\n")
		pre := m.styles.LoadingStyle.Render(m.preloadText)
		content.WriteString(pre)
	}
	// Añadir buffer de streaming en vivo
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
		}
	}
	// Nota: La barra de progreso se mueve al footer para evitar duplicación
	// Chips de estado (al final) cuando no está streameando
	if !m.isStreaming && m.lastResponseDuration > 0 {
		mins := int(m.lastResponseDuration.Minutes())
		secs := int(m.lastResponseDuration.Seconds()) % 60
		chips := []string{
			fmt.Sprintf("⏱ %02d:%02d", mins, secs),
		}
		// Mostrar solo el modelo (sin proveedor) para evitar duplicaciones visuales
		if m.lastResponseModel != "" {
			chips = append(chips, fmt.Sprintf("⚙️ %s", m.lastResponseModel))
		}
		if m.lastResponseTokenApprox > 0 {
			chips = append(chips, fmt.Sprintf("🔤 ~%d tok", m.lastResponseTokenApprox))
		}
		chipLine := m.styles.ChipStyle.Render(strings.Join(chips, "  "))
		animated := lipgloss.NewStyle().MarginLeft(m.chipAnimOffset).Render(chipLine)
		if content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(animated)
	}
	m.viewport.SetContent(content.String())
	if m.viewport.Height > 0 {
		m.viewport.GotoBottom()
	}
}

func (m *appModel) headerView() string {
	if m.isSplashVisible {
		logoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		return lipgloss.JoinVertical(lipgloss.Center, logoStyle.Render(logo), "\nBienvenido a SikmaCode - una nueva experiencia de IA")
	}

	// Kaomoji a la izquierda; a la derecha: ID, LLM (solo modelo), hora y progreso
	kao := m.currentKao
	leftSide := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(kao)
	session := m.sessionName
	llm := "—"
	if m.config != nil {
		llm = m.config.ActiveLLM
		if llm == "" {
			if p, ok := m.config.Providers[m.config.ActiveLLM]; ok {
				llm = p.Name
			}
		}
	}
	if strings.Contains(llm, "/") {
		parts := strings.Split(llm, "/")
		llm = parts[len(parts)-1]
	}
	clock := time.Now().Format("15:04:05")
	rightText := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
		fmt.Sprintf("%s | %s | %s ", session, llm, clock),
	)
	prog := m.prog.View()
	rightSide := lipgloss.JoinHorizontal(lipgloss.Top, rightText, prog)

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

func (m *appModel) thinkingTickCmd() tea.Cmd {
	return tea.Tick(4000*time.Millisecond, func(t time.Time) tea.Msg {
		return thinkingTickMsg{phrase: randomThinkingPhrase()}
	})
}

func (m *appModel) chipAnimTickCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg { return chipAnimTickMsg{} })
}

func (m *appModel) footerView() string {
	help := "[Ctrl+S guardar] [Ctrl+C/Esc salir] [Ayuda: F1] [F2 tema]"
	return m.styles.FooterStyle.Render(help)
}


// Construye el kaomoji con ojos abiertos/cerrados sin cambiar ancho
func (m *appModel) buildKao(open bool) string {
	if open {
		return "≧◉◡◉≦"
	}
	// Variante de ojos cerrados, manteniendo longitud de 5 runas
	return "≧˘◡˘≦"
}


func (m *appModel) kaomojiIdleTickCmd() tea.Cmd {
	// Periodo más largo y natural: 6–11s con jitter
	dur := time.Duration(6000+rand.Intn(5000)) * time.Millisecond
	return tea.Tick(dur, func(t time.Time) tea.Msg {
		// Probabilidad baja de doble parpadeo ocasional (1 de cada 10 aprox.)
		m.kaoDoDouble = rand.Intn(10) == 0
		return kaomojiBlinkMsg{}
	})
}

func (m *appModel) kaomojiResetTickCmd() tea.Cmd {
	// Cierre más lento y suave: 180–300ms
	dur := time.Duration(180+rand.Intn(120)) * time.Millisecond
	return tea.Tick(dur, func(t time.Time) tea.Msg { return kaomojiResetMsg{} })
}

// Segundo parpadeo rápido para el doble parpadeo ocasional
func (m *appModel) kaomojiSecondBlinkTickCmd() tea.Cmd {
	// Pausa breve antes del segundo parpadeo: 220–320ms
	dur := time.Duration(220+rand.Intn(100)) * time.Millisecond
	return tea.Tick(dur, func(t time.Time) tea.Msg { return kaomojiBlinkMsg{} })
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
			"- F2: alternar tema",
			"",
			"Confirmaciones:",
			"- s/y: sí",
			"- n: no",
			"- a: siempre permitir (solo herramientas seguras)",
			"- Nota: comandos críticos requieren doble confirmación",
			"",
			"Estado:",
			"- Header: kaomoji a la izquierda; derecha: ID, LLM, hora y progreso",
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

func (m *appModel) loadingTickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return loadingTickMsg{frame: randomBar(15)}
	})
}

func randomBar(n int) string {
	chars := []rune{'█', '▓', '▒', '░', '─', '━', '╌', '╍', '▌', '▐', '▍', '▎', '▏'}
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func randomAlphaNum(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(letters[rand.Intn(len(letters))])
	}
	return b.String()
}

func randomThinkingPhrase() string {
	phrases := []string{
		"Desplegando mi arsenal tecnológico",
		"Enrutando datos para la mejor respuesta",
		"Activando mi modo desarrollador",
		"Compilando ideas en tiempo real",
		"Sintonizando con el problema",
		"Buscando patrones óptimos",
		"Analizando contexto y dependencias",
		"Refinando lógica de solución",
		"Alineando argumentos y ejemplos",
		"Calculando caminos eficientes",
		"Cargando herramientas de análisis",
		"Indexando conocimiento relevante",
		"Verificando supuestos",
		"Preparando estrategia de respuesta",
		"Trazando el flujo óptimo",
		"Modelando casos edge",
		"Puliendo claridad y precisión",
		"Validando coherencia técnica",
		"Ajustando tono y formato",
		"Estructurando pasos accionables",
		"Curando referencias útiles",
		"Midiendo impacto y riesgos",
		"Seleccionando mejores prácticas",
		"Sincronizando estilo y contenido",
		"Afinando detalles finales",
		"Checklist de calidad listo",
	}
	return phrases[rand.Intn(len(phrases))]
}

type preloadTickMsg struct{ text string }

func (m *appModel) cycleAccentTheme() {
	palette := []string{"178", "81", "135", "201", "86"}
	m.themeAccentIdx = (m.themeAccentIdx + 1) % len(palette)
	color := lipgloss.Color(palette[m.themeAccentIdx])
	m.styles.AgentRole = m.styles.AgentRole.Foreground(color).Bold(true)
	m.styles.ThinkingStyle = m.styles.ThinkingStyle.Foreground(color).Bold(true)
	m.styles.AppStyle = m.styles.AppStyle.BorderForeground(color)
	m.spin.Style = lipgloss.NewStyle().Foreground(color)
}
