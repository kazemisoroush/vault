package retrieve

import (
	"encoding/json"
	"fmt"
	"strings"
)

// answerPayload is the JSON shape the model replies with.
type answerPayload struct {
	Answer string   `json:"answer"`
	IDs    []string `json:"ids"`
}

// parseAnswer pulls the {answer, ids} JSON object out of the model reply, ignoring surrounding text.
func parseAnswer(reply string) (Answer, error) {
	start := strings.Index(reply, "{")
	end := strings.LastIndex(reply, "}")
	if start < 0 || end < start {
		return Answer{}, fmt.Errorf("no answer object in reply")
	}

	var payload answerPayload
	if err := json.Unmarshal([]byte(reply[start:end+1]), &payload); err != nil {
		return Answer{}, fmt.Errorf("unmarshal answer: %w", err)
	}
	return Answer{Text: payload.Answer, IDs: payload.IDs}, nil
}
