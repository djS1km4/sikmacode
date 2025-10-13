package llm

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GenerateResponse se comunica con la API de Google para obtener una respuesta del modelo Gemini.
func GenerateResponse(apiKey string, history []*genai.Content, newMessage string) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("falló al crear el cliente de genai: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-pro")
	cs := model.StartChat()
	
	// La historia de la sesión de chat se establece con los mensajes anteriores.
	cs.History = history

	// Se envía solo el nuevo mensaje.
	resp, err := cs.SendMessage(ctx, genai.Text(newMessage))
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
