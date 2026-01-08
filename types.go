package main

// MCPRequest defines the structure for incoming requests to the MCP endpoint.
type MCPRequest struct {
	Question      string `json:"question"`
	ClaudeOpinion string `json:"claude_opinion"`
}

// MCPResponse defines the structure for responses from the MCP endpoint.
type MCPResponse struct {
	Response string `json:"response"`
}

// ModelOpinions holds all individual model opinions
type ModelOpinions struct {
	Claude      string `json:"claude"`
	Gemini      string `json:"gemini"`
	GPT4        string `json:"gpt4"`
	GeminiError string `json:"gemini_error,omitempty"`
	GPT4Error   string `json:"gpt4_error,omitempty"`
}

// VoteResult represents a model's vote for another model's opinion
type VoteResult struct {
	Voter     string `json:"voter"`      // Who is voting
	ChosenOne string `json:"chosen_one"` // Which model's opinion was chosen
	Reasoning string `json:"reasoning"`  // Why this opinion was chosen
}

// VotingSummary contains the final voting results
type VotingSummary struct {
	Votes      []VoteResult   `json:"votes"`
	VoteCounts map[string]int `json:"vote_counts"`
	Winner     string         `json:"winner"`
	IsTie      bool           `json:"is_tie"`
	TiedModels []string       `json:"tied_models,omitempty"`
}
