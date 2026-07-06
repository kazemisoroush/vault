package extract

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// metaFromReply returns the model's parsed metadata, or an empty map when the model declined.
func metaFromReply(reply string) map[string]string {
	meta, err := parseMeta(reply)
	if err != nil {
		log.Printf("extraction produced no metadata (the model may have declined): %v", err)
		return map[string]string{}
	}
	return meta
}

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
