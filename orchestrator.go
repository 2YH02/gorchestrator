package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/genai"
)

// CallGeminiAPI calls Gemini API directly with a question
func CallGeminiAPI(ctx context.Context, question string) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("environment variable GEMINI_API_KEY not set")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("error creating Gemini client: %w", err)
	}

	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-exp", genai.Text(question), nil)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}

	text := result.Text()
	if text == "" {
		return "", fmt.Errorf("gemini returned empty response")
	}

	fmt.Printf("[DEBUG] Gemini API success, response length: %d\n", len(text))
	return text, nil
}

// CallGPT4API calls OpenAI GPT-4 API directly with a question
func CallGPT4API(ctx context.Context, question string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("environment variable OPENAI_API_KEY not set")
	}

	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type Request struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
	}

	type Choice struct {
		Message Message `json:"message"`
	}

	type Response struct {
		Choices []Choice `json:"choices"`
	}

	reqBody := Request{
		Model: "gpt-4o-mini",
		Messages: []Message{
			{Role: "user", Content: question},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqBytes))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("received non-200 status code: %d, failed to read body: %w", resp.StatusCode, err)
		}
		return "", fmt.Errorf("received non-200 status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var openaiResp Response
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return openaiResp.Choices[0].Message.Content, nil
}

// RunConsensusWorkflow collects opinions from Claude, Gemini, and GPT-4, then determines final solution
func RunConsensusWorkflow(request *MCPRequest) (string, error) {
	// Create context with timeout for all API calls
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex // Protect concurrent writes to opinions
	opinions := &ModelOpinions{
		Claude: request.ClaudeOpinion,
	}

	errs := make(chan error, 2)

	// Call Gemini API
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("[DEBUG] Calling Gemini API...")
		resp, err := CallGeminiAPI(ctx, request.Question)
		mu.Lock()
		if err != nil {
			fmt.Printf("[DEBUG] Gemini API failed: %v\n", err)
			opinions.GeminiError = err.Error()
			errs <- fmt.Errorf("gemini error: %w", err)
		} else {
			fmt.Printf("[DEBUG] Gemini API success, storing response (length: %d)\n", len(resp))
			opinions.Gemini = resp
		}
		mu.Unlock()
	}()

	// Call GPT-4 API
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := CallGPT4API(ctx, request.Question)
		mu.Lock()
		if err != nil {
			opinions.GPT4Error = err.Error()
			errs <- fmt.Errorf("gpt-4 error: %w", err)
		} else {
			opinions.GPT4 = resp
		}
		mu.Unlock()
	}()

	wg.Wait()
	close(errs)

	// Collect errors
	var errorMessages []string
	for err := range errs {
		errorMessages = append(errorMessages, err.Error())
	}

	// Check if we have enough opinions (at least Claude + 1 other)
	validOpinions := 0
	if opinions.Claude != "" {
		validOpinions++
	}
	if opinions.Gemini != "" {
		validOpinions++
	}
	if opinions.GPT4 != "" {
		validOpinions++
	}

	if validOpinions < 2 {
		// If only Claude + 1 other model, that's still acceptable
		return "", fmt.Errorf("insufficient opinions: %d valid, errors: %s", validOpinions, strings.Join(errorMessages, "; "))
	}

	// Log warnings if some models failed
	if len(errorMessages) > 0 {
		fmt.Printf("Warning: Some models failed: %s\n", strings.Join(errorMessages, "; "))
	}

	// If only one opinion besides Claude, return combined result without voting
	if validOpinions == 2 {
		// Simple comparison without voting
		var comparison string
		if opinions.Gemini != "" && opinions.GPT4 == "" {
			comparison = "\n## 📊 Analysis\nOnly Gemini provided feedback alongside Claude's opinion.\n"
		} else if opinions.GPT4 != "" && opinions.Gemini == "" {
			comparison = "\n## 📊 Analysis\nOnly GPT-4o-mini provided feedback alongside Claude's opinion.\n"
		}
		return FormatOpinions(opinions, comparison), nil
	}

	// If all 3 opinions available, run voting
	finalDecision, err := DetermineFinalSolution(opinions)
	if err != nil {
		return "", fmt.Errorf("error determining final solution: %w", err)
	}

	return FormatOpinions(opinions, finalDecision), nil
}

// FormatOpinions formats all opinions and final decision into readable output
func FormatOpinions(opinions *ModelOpinions, finalDecision string) string {
	var result strings.Builder

	result.WriteString("# 🤖 AI Model Opinions\n\n")

	if opinions.Claude != "" {
		result.WriteString("## 💬 Claude's Opinion\n")
		result.WriteString(opinions.Claude)
		result.WriteString("\n\n---\n\n")
	}

	if opinions.Gemini != "" {
		result.WriteString("## 🔷 Gemini's Opinion\n")
		result.WriteString(opinions.Gemini)
		result.WriteString("\n\n---\n\n")
	} else if opinions.GeminiError != "" {
		result.WriteString("## 🔷 Gemini's Opinion\n")
		result.WriteString("❌ **Error occurred:**\n```\n")
		result.WriteString(opinions.GeminiError)
		result.WriteString("\n```\n\n---\n\n")
	}

	if opinions.GPT4 != "" {
		result.WriteString("## 🟢 GPT-4's Opinion\n")
		result.WriteString(opinions.GPT4)
		result.WriteString("\n\n---\n\n")
	} else if opinions.GPT4Error != "" {
		result.WriteString("## 🟢 GPT-4's Opinion\n")
		result.WriteString("❌ **Error occurred:**\n```\n")
		result.WriteString(opinions.GPT4Error)
		result.WriteString("\n```\n\n---\n\n")
	}

	if finalDecision != "" {
		result.WriteString("## 🏆 Final Decision\n")
		result.WriteString(finalDecision)
		result.WriteString("\n")
	}

	return result.String()
}
