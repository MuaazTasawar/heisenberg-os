package legal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type SaulResponse struct {
	Advice  string `json:"advice"`
	Risk    string `json:"risk"`
	Defense string `json:"defense"`
}

func ConsultSaul(charge, evidence, playerContext string) (*SaulResponse, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	prompt := fmt.Sprintf(`You are Saul Goodman (Jimmy McGill) from Breaking Bad. 
A client comes to you with the following legal problem.
Charge: %s
Evidence against them: %s
Their situation: %s

Respond in character as Saul — fast-talking, creative, morally flexible. 
Give: 1) Your legal advice 2) The risk level (low/medium/high/critical) 3) Your defense strategy.
Keep it under 200 words. Stay in character.`, charge, evidence, playerContext)

	body := map[string]interface{}{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 500,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	return &SaulResponse{
		Advice:  text,
		Risk:    "medium",
		Defense: "See Saul's advice above",
	}, nil
}
