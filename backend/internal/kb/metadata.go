package kb

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/document"
)

// Metadata keys the ingestion side stamps on each document and the searcher reads back, so a
// retrieved passage can be tied to the file it came from.
const (
	MetaFileID   = "fileId"
	MetaFileName = "fileName"
)

// MetadataSidecar returns the Knowledge Base metadata sidecar JSON for a file: the attributes the
// managed data source stamps on every passage it indexes from the object, so a retrieved passage
// carries the file id and name. The ingestion side writes this next to the stored object.
func MetadataSidecar(fileID string, fileName string) ([]byte, error) {
	sidecar := struct {
		MetadataAttributes map[string]string `json:"metadataAttributes"`
	}{MetadataAttributes: map[string]string{MetaFileID: fileID, MetaFileName: fileName}}
	body, err := json.Marshal(sidecar)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata sidecar for %q: %w", fileID, err)
	}
	return body, nil
}

// metaString reads a string metadata value the ingestion side stamped, or "" when it is absent or
// not a string.
func metaString(meta map[string]document.Interface, key string) string {
	doc, ok := meta[key]
	if !ok {
		return ""
	}
	var value string
	if err := doc.UnmarshalSmithyDocument(&value); err != nil {
		return ""
	}
	return value
}
