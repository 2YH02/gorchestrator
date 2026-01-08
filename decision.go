package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"google.golang.org/genai"
)

// EvaluationResponse represents the JSON response from a model's evaluation
type EvaluationResponse struct {
	ChosenModel string `json:"chosen_model"`
	Reasoning   string `json:"reasoning"`
}

// CallModelForVote asks a model to vote between two other models' opinions
func CallModelForVote(ctx context.Context, voter string, opinions *ModelOpinions, excludeSelf string) (*VoteResult, error) {
	// Build the voting prompt with the two other opinions
	candidates := []struct {
		name    string
		opinion string
	}{}

	if opinions.Claude != "" && voter != "Claude" && excludeSelf != "Claude" {
		candidates = append(candidates, struct {
			name    string
			opinion string
		}{"Claude", opinions.Claude})
	}
	if opinions.Gemini != "" && voter != "Gemini" && excludeSelf != "Gemini" {
		candidates = append(candidates, struct {
			name    string
			opinion string
		}{"Gemini", opinions.Gemini})
	}
	if opinions.GPT4 != "" && voter != "GPT-4" && excludeSelf != "GPT-4" {
		candidates = append(candidates, struct {
			name    string
			opinion string
		}{"GPT-4", opinions.GPT4})
	}

	if len(candidates) < 2 {
		return nil, fmt.Errorf("not enough candidates for %s to vote on", voter)
	}

	var opinionsText strings.Builder
	validNames := []string{}
	for _, c := range candidates {
		opinionsText.WriteString(fmt.Sprintf("**%s's Opinion:**\n%s\n\n", c.name, c.opinion))
		validNames = append(validNames, c.name)
	}

	namesText := strings.Join(validNames, " or ")

	prompt := fmt.Sprintf(`You are evaluating two AI model opinions on the same question. Your task is to objectively determine which opinion is better.

Here are the opinions:

%s

Evaluate based on:
- Correctness and accuracy
- Completeness and depth
- Clarity and organization
- Practical usefulness

Output ONLY a JSON object:
{
  "chosen_model": "The name of the better model (%s)",
  "reasoning": "Brief explanation of why this opinion is better"
}

Do not include any text outside the JSON object.`, opinionsText.String(), namesText)

	var response string
	var err error

	switch voter {
	case "Gemini":
		response, err = callGeminiForEvaluation(ctx, prompt)
	case "GPT-4":
		response, err = callGPT4ForEvaluation(ctx, prompt)
	case "Claude":
		// Claude는 이미 Claude Code에서 실행 중이므로 다른 방법 필요
		// 일단 에러 반환
		return nil, fmt.Errorf("claude cannot vote (already running in Claude Code)")
	default:
		return nil, fmt.Errorf("unknown voter: %s", voter)
	}

	if err != nil {
		return nil, fmt.Errorf("error getting vote from %s: %w", voter, err)
	}

	// Parse JSON response
	jsonString := strings.TrimSpace(response)
	if after, found := strings.CutPrefix(jsonString, "```json"); found {
		jsonString = strings.TrimSuffix(after, "```")
		jsonString = strings.TrimSpace(jsonString)
	} else if after, found := strings.CutPrefix(jsonString, "```"); found {
		jsonString = strings.TrimSuffix(after, "```")
		jsonString = strings.TrimSpace(jsonString)
	}

	var evalResp EvaluationResponse
	if err := json.Unmarshal([]byte(jsonString), &evalResp); err != nil {
		return nil, fmt.Errorf("error parsing vote from %s: %w\nResponse: %s", voter, err, jsonString)
	}

	return &VoteResult{
		Voter:     voter,
		ChosenOne: evalResp.ChosenModel,
		Reasoning: evalResp.Reasoning,
	}, nil
}

func callGeminiForEvaluation(ctx context.Context, prompt string) (string, error) {
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

	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash-exp", genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("error generating content: %w", err)
	}

	text := result.Text()
	if text == "" {
		return "", fmt.Errorf("gemini returned empty response")
	}

	fmt.Printf("[DEBUG] Gemini evaluation success, response length: %d\n", len(text))
	return text, nil
}

func callGPT4ForEvaluation(ctx context.Context, prompt string) (string, error) {
	// Reuse the CallGPT4API function from orchestrator
	return CallGPT4API(ctx, prompt)
}

// ConductVoting runs the peer voting process
func ConductVoting(opinions *ModelOpinions) (*VotingSummary, error) {
	ctx := context.Background()
	var wg sync.WaitGroup
	votesChan := make(chan *VoteResult, 3)
	errsChan := make(chan error, 3)

	// Gemini votes (between Claude and GPT-4)
	if opinions.Gemini != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vote, err := CallModelForVote(ctx, "Gemini", opinions, "Gemini")
			if err != nil {
				errsChan <- err
				return
			}
			votesChan <- vote
		}()
	}

	// GPT-4 votes (between Claude and Gemini)
	if opinions.GPT4 != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vote, err := CallModelForVote(ctx, "GPT-4", opinions, "GPT-4")
			if err != nil {
				errsChan <- err
				return
			}
			votesChan <- vote
		}()
	}

	// Note: Claude는 투표할 수 없음 (Claude Code에서 이미 실행 중)
	// 2개 모델만 투표하는 것으로 진행

	wg.Wait()
	close(votesChan)
	close(errsChan)

	// Collect errors
	var errors []string
	for err := range errsChan {
		errors = append(errors, err.Error())
	}

	// Collect votes
	var votes []VoteResult
	for vote := range votesChan {
		votes = append(votes, *vote)
	}

	if len(votes) == 0 {
		return nil, fmt.Errorf("no votes collected. Errors: %s", strings.Join(errors, "; "))
	}

	// Count votes
	voteCounts := make(map[string]int)
	voteCounts["Claude"] = 0
	voteCounts["Gemini"] = 0
	voteCounts["GPT-4"] = 0

	for _, vote := range votes {
		voteCounts[vote.ChosenOne]++
	}

	// Determine winner
	maxVotes := 0
	var winner string
	var tiedModels []string

	for model, count := range voteCounts {
		if count > maxVotes {
			maxVotes = count
			winner = model
			tiedModels = []string{model}
		} else if count == maxVotes && count > 0 {
			tiedModels = append(tiedModels, model)
		}
	}

	isTie := len(tiedModels) > 1

	return &VotingSummary{
		Votes:      votes,
		VoteCounts: voteCounts,
		Winner:     winner,
		IsTie:      isTie,
		TiedModels: tiedModels,
	}, nil
}

// DetermineFinalSolution uses peer voting to determine the best opinion
func DetermineFinalSolution(opinions *ModelOpinions) (string, error) {
	votingSummary, err := ConductVoting(opinions)
	if err != nil {
		return "", fmt.Errorf("error conducting voting: %w", err)
	}

	// Format the result
	var result strings.Builder

	result.WriteString("## 🗳️ Voting Results\n\n")

	// Show individual votes
	for _, vote := range votingSummary.Votes {
		result.WriteString(fmt.Sprintf("**%s's Vote:** %s\n", vote.Voter, vote.ChosenOne))
		result.WriteString(fmt.Sprintf("*Reasoning:* %s\n\n", vote.Reasoning))
	}

	// Show vote counts
	result.WriteString("### Vote Count:\n")
	for model, count := range votingSummary.VoteCounts {
		emoji := ""
		switch model {
		case "Claude":
			emoji = "💬"
		case "Gemini":
			emoji = "🔷"
		case "GPT-4":
			emoji = "🟢"
		}
		result.WriteString(fmt.Sprintf("- %s **%s**: %d vote(s)\n", emoji, model, count))
	}
	result.WriteString("\n")

	// Show winner
	if votingSummary.IsTie {
		result.WriteString(fmt.Sprintf("### 🤝 Result: Tie between %s\n", strings.Join(votingSummary.TiedModels, " and ")))
		result.WriteString("All tied opinions are equally valuable!\n")
	} else {
		winnerEmoji := ""
		switch votingSummary.Winner {
		case "Claude":
			winnerEmoji = "💬"
		case "Gemini":
			winnerEmoji = "🔷"
		case "GPT-4":
			winnerEmoji = "🟢"
		}
		result.WriteString(fmt.Sprintf("### 🏆 Winner: %s **%s**\n", winnerEmoji, votingSummary.Winner))
		result.WriteString(fmt.Sprintf("%s's opinion received the most votes from peer models!\n", votingSummary.Winner))
	}

	return result.String(), nil
}
