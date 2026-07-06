package extract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
)

// officeProbe reads the built-in core properties of an OOXML office document.
type officeProbe struct{}

// Supports reports whether the content is an OOXML office document.
func (officeProbe) Supports(contentType string) bool {
	return isOffice(contentType)
}

// Probe returns the office document's core properties (author, title, subject, dates).
func (officeProbe) Probe(content []byte) map[string]string {
	meta := map[string]string{}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return meta
	}
	for _, file := range reader.File {
		if file.Name != "docProps/core.xml" {
			continue
		}
		if props := parseCoreProps(file); props != nil {
			return props
		}
	}
	return meta
}

// parseCoreProps decodes docProps/core.xml into a flat metadata map, or nil on failure.
func parseCoreProps(file *zip.File) map[string]string {
	rc, err := file.Open()
	if err != nil {
		return nil
	}
	defer func() { _ = rc.Close() }()

	var props coreProperties
	if err := xml.NewDecoder(io.LimitReader(rc, maxPartBytes)).Decode(&props); err != nil {
		return nil
	}

	meta := map[string]string{}
	setIf(meta, "author", props.Creator)
	setIf(meta, "title", props.Title)
	setIf(meta, "subject", props.Subject)
	setIf(meta, "created", dateOnly(props.Created))
	setIf(meta, "modified", dateOnly(props.Modified))
	return meta
}
