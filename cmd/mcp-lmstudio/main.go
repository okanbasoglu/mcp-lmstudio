package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName      = "lmstudio-bridge"
	ServerVersion   = "0.1.0"
	LMStudioAPIBase = "http://127.0.0.1:1234"
	LogFilePath     = "lmstudio_audit.log"
)

type ModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type Integration struct {
	Type         string   `json:"type"`
	ID           string   `json:"id"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
}

type ChatCompletionRequest struct {
	Model         string        `json:"model"`
	Input         string        `json:"input"`
	Temperature   float64       `json:"temperature,omitempty"`
	ContextLength int           `json:"context_length,omitempty"`
	Integrations  []Integration `json:"integrations,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	Output string `json:"output"`
	Model  string `json:"model,omitempty"`
}

type EmptyArgs struct{}

type ChatCompletionArgs struct {
	Prompt        string  `json:"prompt" jsonschema:"The user's prompt to send to the model"`
	SystemPrompt  string  `json:"system_prompt,omitempty" jsonschema:"Optional system instructions for the model"`
	Temperature   float64 `json:"temperature,omitempty" jsonschema:"Controls randomness (0.0 to 1.0)"`
	ContextLength int     `json:"context_length,omitempty" jsonschema:"Maximum context length in tokens"`
}

func getDefaultIntegrations() []Integration {
	integrationsJSON := os.Getenv("LMSTUDIO_INTEGRATIONS")
	if integrationsJSON == "" {
		return nil
	}

	var integrations []Integration
	if err := json.Unmarshal([]byte(integrationsJSON), &integrations); err != nil {
		log.Printf("Warning: Failed to parse LMSTUDIO_INTEGRATIONS: %v", err)
		return nil
	}

	return integrations
}

func getModelName() string {
	if model := os.Getenv("LMSTUDIO_MODEL"); model != "" {
		return model
	}
	return "default"
}

func getDefaultContextLength() int {
	if contextLengthStr := os.Getenv("LMSTUDIO_CONTEXT_LENGTH"); contextLengthStr != "" {
		var contextLength int
		if _, err := fmt.Sscanf(contextLengthStr, "%d", &contextLength); err == nil && contextLength > 0 {
			return contextLength
		}
		log.Printf("Warning: Invalid LMSTUDIO_CONTEXT_LENGTH value: %s, using default 2048", contextLengthStr)
	}
	return 2048
}

func getAuthHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if token := os.Getenv("LMSTUDIO_API_TOKEN"); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}

func addHeadersToRequest(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

func logError(logger *log.Logger, message string) {
	logger.Printf("ERROR: %s", message)
}

func logInfo(logger *log.Logger, message string) {
	logger.Printf("INFO: %s", message)
}

func main() {
	logFile, err := os.OpenFile(LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("could not open log file: %v", err))
	}
	defer logFile.Close()
	logger := log.New(logFile, "AUDIT: ", log.LstdFlags)

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    ServerName,
			Version: ServerVersion,
		},
		&mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{
				Tools: &mcp.ToolCapabilities{ListChanged: true},
			},
		},
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "health_check",
		Description: "Check if LM Studio API is accessible.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
		logInfo(logger, "Executing health_check")

		client := &http.Client{Timeout: 10 * time.Second}
		httpReq, err := http.NewRequestWithContext(ctx, "GET", LMStudioAPIBase+"/v1/models", nil)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create request: %w", err)
		}
		addHeadersToRequest(httpReq, getAuthHeaders())

		resp, err := client.Do(httpReq)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error connecting to LM Studio API: %v", err),
					},
				},
			}, nil, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: "LM Studio API is running and accessible.",
					},
				},
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("LM Studio API returned status code %d.", resp.StatusCode),
				},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_models",
		Description: "List all available models in LM Studio.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args EmptyArgs) (*mcp.CallToolResult, any, error) {
		logInfo(logger, "Executing list_models")

		client := &http.Client{Timeout: 10 * time.Second}
		httpReq, err := http.NewRequestWithContext(ctx, "GET", LMStudioAPIBase+"/v1/models", nil)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create request: %w", err)
		}
		addHeadersToRequest(httpReq, getAuthHeaders())

		resp, err := client.Do(httpReq)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error listing models: %v", err),
					},
				},
			}, nil, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Failed to fetch models. Status code: %d", resp.StatusCode),
					},
				},
			}, nil, nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response: %w", err)
		}

		var modelsResp ModelsResponse
		if err := json.Unmarshal(body, &modelsResp); err != nil {
			return nil, nil, fmt.Errorf("failed to parse response: %w", err)
		}

		if len(modelsResp.Data) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: "No models found in LM Studio.",
					},
				},
			}, nil, nil
		}

		result := "Available models in LM Studio:\n\n"
		for _, model := range modelsResp.Data {
			result += fmt.Sprintf("- %s\n", model.ID)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: result,
				},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "chat",
		Description: "Generate a completion from the current LM Studio model.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ChatCompletionArgs) (*mcp.CallToolResult, any, error) {
		logInfo(logger, fmt.Sprintf("Executing chat with prompt: %s", args.Prompt))

		temperature := args.Temperature
		if temperature == 0 {
			temperature = 0.7
		}

		contextLength := args.ContextLength
		if contextLength == 0 {
			contextLength = getDefaultContextLength()
		}

		// Combine system prompt and user prompt into input
		input := args.Prompt
		if args.SystemPrompt != "" {
			input = fmt.Sprintf("System: %s\n\nUser: %s", args.SystemPrompt, args.Prompt)
		}

		chatReq := ChatCompletionRequest{
			Model:         getModelName(),
			Input:         input,
			Temperature:   temperature,
			ContextLength: contextLength,
		}

		integrations := getDefaultIntegrations()
		if len(integrations) > 0 {
			chatReq.Integrations = integrations
		}

		jsonData, err := json.Marshal(chatReq)
		if err != nil {
			logError(logger, fmt.Sprintf("Failed to marshal request: %v", err))
			return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		logInfo(logger, fmt.Sprintf("Sending request to LM Studio, integrations: %v", chatReq.Integrations))

		client := &http.Client{Timeout: 60 * time.Second}
		httpReq, err := http.NewRequestWithContext(ctx, "POST", LMStudioAPIBase+"/api/v1/chat", bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create request: %w", err)
		}
		addHeadersToRequest(httpReq, getAuthHeaders())

		resp, err := client.Do(httpReq)
		if err != nil {
			logError(logger, fmt.Sprintf("Error generating completion: %v", err))
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error generating completion: %v", err),
					},
				},
			}, nil, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			logError(logger, fmt.Sprintf("LM Studio API error: %d, Body: %s", resp.StatusCode, string(body)))
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error: LM Studio returned status code %d. Response: %s", resp.StatusCode, string(body)),
					},
				},
			}, nil, nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logError(logger, fmt.Sprintf("Failed to read response: %v", err))
			return nil, nil, fmt.Errorf("failed to read response: %w", err)
		}

		var chatResp ChatCompletionResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			logError(logger, fmt.Sprintf("Failed to parse response: %v", err))
			return nil, nil, fmt.Errorf("failed to parse response: %w", err)
		}

		logInfo(logger, "Received response from LM Studio")

		if chatResp.Output == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: "Error: Empty response from model",
					},
				},
			}, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: chatResp.Output,
				},
			},
		}, nil, nil
	})

	transport := &mcp.LoggingTransport{
		Transport: &mcp.StdioTransport{},
		Writer:    logFile,
	}

	session, err := server.Connect(context.Background(), transport, nil)
	if err != nil {
		logger.Fatalf("Connection error: %v", err)
	}

	if err := session.Wait(); err != nil {
		logger.Printf("Session closed with error: %v", err)
	}
}
