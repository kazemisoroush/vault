package extract

import (
	"bytes"
	"strings"

	"rsc.io/pdf"
)

// pdfProbe reads the document information dictionary of a PDF.
type pdfProbe struct{}

// Supports reports whether the content is a PDF.
func (pdfProbe) Supports(contentType string) bool {
	return contentType == "application/pdf"
}

// Probe returns the PDF's /Info metadata (author, title, dates), recovering if the parser panics.
func (pdfProbe) Probe(content []byte) (meta map[string]string) {
	meta = map[string]string{}
	defer func() {
		if recover() != nil {
			meta = map[string]string{}
		}
	}()
	reader, err := pdf.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return meta
	}
	info := reader.Trailer().Key("Info")
	setIf(meta, "author", info.Key("Author").Text())
	setIf(meta, "title", info.Key("Title").Text())
	setIf(meta, "subject", info.Key("Subject").Text())
	setIf(meta, "created", pdfDate(info.Key("CreationDate").Text()))
	setIf(meta, "modified", pdfDate(info.Key("ModDate").Text()))
	return meta
}

// pdfDate turns a PDF date string (D:YYYYMMDD...) into an ISO date, or "" when it cannot.
func pdfDate(value string) string {
	digits, ok := strings.CutPrefix(value, "D:")
	if !ok || len(digits) < 8 {
		return ""
	}
	return digits[0:4] + "-" + digits[4:6] + "-" + digits[6:8]
}
