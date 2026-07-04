package retrieve

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseIDs pulls the JSON array of ids out of the model reply, ignoring any surrounding text.
func parseIDs(reply string) ([]string, error) {
	start := strings.Index(reply, "[")
	end := strings.LastIndex(reply, "]")
	if start < 0 || end < start {
		return nil, fmt.Errorf("no id array in reply")
	}
	var ids []string
	if err := json.Unmarshal([]byte(reply[start:end+1]), &ids); err != nil {
		return nil, fmt.Errorf("unmarshal ids: %w", err)
	}
	return ids, nil
}
