package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// MCP JSON-RPC structures
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Tools Tools `json:"tools"`
}

type Tools struct{}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolResult struct {
	Content []ContentItem `json:"content"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPServer handles MCP protocol over stdio
func RunMCPServer() {
	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(writer, nil, -32700, "Parse error: "+err.Error())
			continue
		}

		handleRequest(writer, &req)
	}
}

func handleRequest(writer *bufio.Writer, req *JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		handleInitialize(writer, req)
	case "tools/list":
		handleToolsList(writer, req)
	case "tools/call":
		handleToolCall(writer, req)
	default:
		sendError(writer, req.ID, -32601, fmt.Sprintf("Unknown method: %s", req.Method))
	}
}

func handleInitialize(writer *bufio.Writer, req *JSONRPCRequest) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: Tools{},
		},
		ServerInfo: ServerInfo{
			Name:    "gorchestrator",
			Version: "1.0.0",
		},
	}
	sendResponse(writer, req.ID, result)
}

func handleToolsList(writer *bufio.Writer, req *JSONRPCRequest) {
	tools := map[string]interface{}{
		"tools": []ToolDefinition{
			{
				Name:        "ask_ai_consensus",
				Description: "Ask a question to multiple AI models (Claude, Gemini, GPT-4) and get their opinions with democratic voting. Each AI model will evaluate the others' opinions and vote for the best one. Returns all opinions and voting results.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"question": {
							Type:        "string",
							Description: "The question to ask all AI models",
						},
						"claude_opinion": {
							Type:        "string",
							Description: "Your (Claude's) initial opinion on the question. This will be shared with other AI models for evaluation.",
						},
					},
					Required: []string{"question", "claude_opinion"},
				},
			},
		},
	}
	sendResponse(writer, req.ID, tools)
}

func handleToolCall(writer *bufio.Writer, req *JSONRPCRequest) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		sendError(writer, req.ID, -32602, "Invalid params: "+err.Error())
		return
	}

	if params.Name != "ask_ai_consensus" {
		sendError(writer, req.ID, -32601, fmt.Sprintf("Unknown tool: %s", params.Name))
		return
	}

	question, ok := params.Arguments["question"].(string)
	if !ok {
		sendError(writer, req.ID, -32602, "Missing or invalid 'question' argument")
		return
	}

	claudeOpinion, ok := params.Arguments["claude_opinion"].(string)
	if !ok {
		sendError(writer, req.ID, -32602, "Missing or invalid 'claude_opinion' argument")
		return
	}

	// Call the consensus workflow
	mcpReq := &MCPRequest{
		Question:      question,
		ClaudeOpinion: claudeOpinion,
	}

	response, err := RunConsensusWorkflow(mcpReq)
	if err != nil {
		sendError(writer, req.ID, -32603, fmt.Sprintf("Failed to run consensus workflow: %v", err))
		return
	}

	result := ToolResult{
		Content: []ContentItem{
			{
				Type: "text",
				Text: response,
			},
		},
	}
	sendResponse(writer, req.ID, result)
}

func sendResponse(writer *bufio.Writer, id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	writer.Write(data)
	writer.WriteByte('\n')
	writer.Flush()
}

func sendError(writer *bufio.Writer, id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	data, _ := json.Marshal(resp)
	writer.Write(data)
	writer.WriteByte('\n')
	writer.Flush()
}
