package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/generative-ai-go/genai"
	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
	"google.golang.org/api/iterator"
)

// GenerateResponse se comunica con la API de Google para obtener una respuesta del modelo Gemini.
func GenerateResponse(apiKey, modelName, systemPrompt string, history []*genai.Content, newMessage string) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("falló al crear el cliente de genai: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel(modelName)

	// Usar SystemInstruction para establecer el prompt de sistema de forma apropiada
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}

	// Iniciar sesión de chat con el historial existente (si lo hay)
	cs := model.StartChat()
	cs.History = history

	// Se envía solo el nuevo mensaje.
	resp, err := cs.SendMessage(context.Background(), genai.Text(newMessage))
	if err != nil {
		return "", fmt.Errorf("falló al enviar el mensaje: %w", err)
	}

	// Extraer y devolver el contenido de la respuesta.
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		if txt, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
			return string(txt), nil
		}
	}

	return "", fmt.Errorf("no se encontró contenido de texto en la respuesta")
}

// StreamIterator define una interfaz de iterador de chunks agnóstico al proveedor.
type StreamIterator interface {
	Next() (string, error)
}

// StreamClient define una interfaz simple para cerrar recursos de streaming.
type StreamClient interface {
	Close() error
}

// StartStream inicia un streaming según el proveedor seleccionado.
// providerID: "gemini" | "google-gemini" | "openai" | ...
func StartStream(providerID, apiKey, modelName, systemPrompt string, history []*genai.Content, newMessage string) (StreamClient, StreamIterator, error) {
	switch strings.ToLower(providerID) {
	case "gemini", "google-gemini":
		return startGeminiStream(apiKey, modelName, systemPrompt, history, newMessage)
	case "openai":
		return startOpenAIStream(apiKey, modelName, systemPrompt, history, newMessage)
	default:
		return nil, nil, fmt.Errorf("proveedor '%s' no soportado aún", providerID)
	}
}

// ==== Gemini (Google Generative AI) ====

type genaiStreamClient struct{ c *genai.Client }

func (g *genaiStreamClient) Close() error { g.c.Close(); return nil }

type genaiStreamIter struct{ it *genai.GenerateContentResponseIterator }

func (gi *genaiStreamIter) Next() (string, error) {
	resp, err := gi.it.Next()
	if err != nil {
		// Unificar señal de fin de iteración a io.EOF
		if errors.Is(err, io.EOF) || errors.Is(err, iterator.Done) {
			return "", io.EOF
		}
		return "", err
	}
	var b strings.Builder
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, p := range resp.Candidates[0].Content.Parts {
			if t, ok := p.(genai.Text); ok {
				b.WriteString(string(t))
			}
		}
	}
	return b.String(), nil
}

func startGeminiStream(apiKey, modelName, systemPrompt string, history []*genai.Content, newMessage string) (StreamClient, StreamIterator, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, nil, fmt.Errorf("falló al crear el cliente de genai: %w", err)
	}
	model := client.GenerativeModel(modelName)
	model.SystemInstruction = &genai.Content{Parts: []genai.Part{genai.Text(systemPrompt)}}
	cs := model.StartChat()
	cs.History = history
	iter := cs.SendMessageStream(ctx, genai.Text(newMessage))
	return &genaiStreamClient{c: client}, &genaiStreamIter{it: iter}, nil
}

// ==== OpenAI (go-openai) ====

type openaiStreamClient struct{ stream *openai.ChatCompletionStream }

func (c *openaiStreamClient) Close() error { return c.stream.Close() }

type openaiStreamIter struct{ stream *openai.ChatCompletionStream }

func (oi *openaiStreamIter) Next() (string, error) {
	resp, err := oi.stream.Recv()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", io.EOF
		}
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Delta.Content, nil
}

func startOpenAIStream(apiKey, modelName, systemPrompt string, history []*genai.Content, newMessage string) (StreamClient, StreamIterator, error) {
	ctx := context.Background()
	client := openai.NewClient(apiKey)
	msgs := convertGeminiHistoryToOpenAI(systemPrompt, history, newMessage)
	req := openai.ChatCompletionRequest{
		Model:    modelName,
		Messages: msgs,
		Stream:   true,
	}
	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	return &openaiStreamClient{stream: stream}, &openaiStreamIter{stream: stream}, nil
}

func convertGeminiHistoryToOpenAI(systemPrompt string, history []*genai.Content, newMessage string) []openai.ChatCompletionMessage {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}
	for _, c := range history {
		var text string
		for _, p := range c.Parts {
			if t, ok := p.(genai.Text); ok {
				text += string(t)
			}
		}
		role := openai.ChatMessageRoleUser
		if strings.ToLower(c.Role) == "model" || strings.ToLower(c.Role) == "assistant" {
			role = openai.ChatMessageRoleAssistant
		}
		msgs = append(msgs, openai.ChatCompletionMessage{Role: role, Content: text})
	}
	msgs = append(msgs, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: newMessage})
	return msgs
}