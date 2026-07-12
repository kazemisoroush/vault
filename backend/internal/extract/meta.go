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

// transcriptionFromReply parses a transcribing reply's {"meta": ..., "text": ...} object,
// returning empty values when the model declined so ingest still lands the file record.
func transcriptionFromReply(reply string) (map[string]string, string) {
	start := strings.Index(reply, "{")
	end := strings.LastIndex(reply, "}")
	if start < 0 || end < 0 || end < start {
		log.Printf("transcription produced no JSON (the model may have declined)")
		return map[string]string{}, ""
	}

	var parsed struct {
		Meta map[string]string `json:"meta"`
		Text string            `json:"text"`
	}
	if err := json.Unmarshal([]byte(reply[start:end+1]), &parsed); err != nil {
		log.Printf("transcription reply did not decode: %v", err)
		return map[string]string{}, ""
	}
	if parsed.Meta == nil {
		parsed.Meta = map[string]string{}
	}
	return parsed.Meta, parsed.Text
}
