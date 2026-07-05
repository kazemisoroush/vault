package extract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// officeContentTypes are the OOXML (zipped) formats whose bytes must be decoded before the model
// can read them.
var officeContentTypes = map[string]bool{
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
}

// isOffice reports whether a content type is an OOXML office document.
func isOffice(contentType string) bool {
	return officeContentTypes[contentType]
}

// officeText unzips an OOXML file and returns the text from its content parts, tags stripped.
func officeText(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("open office zip: %w", err)
	}

	var out strings.Builder
	for _, file := range reader.File {
		if !isContentPart(file.Name) {
			continue
		}
		if text, err := xmlText(file); err == nil {
			out.WriteString(text)
			out.WriteByte('\n')
		}
	}
	return strings.TrimSpace(out.String()), nil
}

// isContentPart reports whether an OOXML zip entry holds document text (Word, Excel strings, slides).
func isContentPart(name string) bool {
	switch {
	case name == "word/document.xml", name == "xl/sharedStrings.xml":
		return true
	case strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml"):
		return true
	}
	return false
}

// xmlText returns the concatenated character data of a zip entry, dropping the XML markup.
func xmlText(file *zip.File) (string, error) {
	rc, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open part %q: %w", file.Name, err)
	}
	defer func() { _ = rc.Close() }()

	decoder := xml.NewDecoder(rc)
	var out strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read part %q: %w", file.Name, err)
		}
		if data, ok := token.(xml.CharData); ok {
			out.Write(data)
			out.WriteByte(' ')
		}
	}
	return out.String(), nil
}
