package agent

import (
	"encoding/json"
	"strings"
)

// finalPayload is the JSON object the model ends with. FileIDs are the ids of the vault files
// the model used to answer, so the caller can load and cite them.
type finalPayload struct {
	Answer  string   `json:"answer"`
	FileIDs []string `json:"fileIds"`
}

// parseFinal reads the model's closing {answer, fileIds} object. When the reply is not that
// object, the whole reply is treated as the answer with no cited file ids, so a stray reply
// still returns something usable.
func parseFinal(reply string) (string, []string) {
	start := strings.Index(reply, "{")
	end := strings.LastIndex(reply, "}")
	if start < 0 || end < start {
		return strings.TrimSpace(reply), nil
	}

	var payload finalPayload
	if err := json.Unmarshal([]byte(reply[start:end+1]), &payload); err != nil {
		return strings.TrimSpace(reply), nil
	}
	return payload.Answer, payload.FileIDs
}
