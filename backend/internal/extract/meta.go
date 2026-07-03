package extract

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseMeta extracts the JSON object from a model reply into a flat string map.
func parseMeta(reply string) (map[string]string, error) {
	start := strings.Index(reply, "{")
	end := strings.LastIndex(reply, "}")
	if start < 0 || end < 0 || end < start {
		return nil, fmt.Errorf("no JSON object in model reply")
	}

	meta := map[string]string{}
	if err := json.Unmarshal([]byte(reply[start:end+1]), &meta); err != nil {
		return nil, fmt.Errorf("decode model reply: %w", err)
	}
	return meta, nil
}
