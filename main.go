package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func mcpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var mcpReq MCPRequest
	if err := json.Unmarshal(body, &mcpReq); err != nil {
		http.Error(w, "Error unmarshaling request body", http.StatusBadRequest)
		return
	}

	log.Printf("Received question: %s", mcpReq.Question)
	log.Printf("Claude opinion length: %d chars", len(mcpReq.ClaudeOpinion))

	// Validate required fields
	if mcpReq.Question == "" {
		http.Error(w, "Question is required", http.StatusBadRequest)
		return
	}

	if mcpReq.ClaudeOpinion == "" {
		http.Error(w, "Claude opinion is required", http.StatusBadRequest)
		return
	}

	response, err := RunConsensusWorkflow(&mcpReq)
	if err != nil {
		log.Printf("Error in consensus workflow: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	mcpResp := MCPResponse{
		Response: response,
	}

	respBody, err := json.Marshal(mcpResp)
	if err != nil {
		http.Error(w, "Error marshaling response body", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

func main() {
	// Load environment variables
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	// Check if running as MCP server (stdio mode)
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		// Run as MCP server using stdio
		RunMCPServer()
		return
	}

	// Run as HTTP server (default)
	http.HandleFunc("/orchestrate", mcpHandler)

	fmt.Println("Server listening on port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
