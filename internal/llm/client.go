package llm

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
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

// StartStream inicia un flujo de respuesta en streaming y devuelve el cliente y el iterador.
func StartStream(apiKey, modelName, systemPrompt string, history []*genai.Content, newMessage string) (*genai.Client, *genai.GenerateContentResponseIterator, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, nil, fmt.Errorf("falló al crear el cliente de genai: %w", err)
	}

	model := client.GenerativeModel(modelName)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	cs := model.StartChat()
	cs.History = history

	iter := cs.SendMessageStream(ctx, genai.Text(newMessage))
	return client, iter, nil
}