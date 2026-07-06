package extract

import (
	"maps"
	"strings"
)

//go:generate go tool mockgen -source=probe.go -destination=../mocks/probe_mock.go -package=mocks

// Probe reads embedded metadata from the bytes of the content types it supports.
type Probe interface {
	Supports(contentType string) bool
	Probe(content []byte) map[string]string
}

// probes lists the embedded-metadata readers, one per file type, and is the only place a type is registered.
var probes = []Probe{exifProbe{}, officeProbe{}, pdfProbe{}}

// embeddedMeta returns best-effort metadata from the file's own bytes, gathered from every probe
// that supports its content type.
func embeddedMeta(content []byte, contentType string) map[string]string {
	meta := map[string]string{}
	for _, probe := range probes {
		if probe.Supports(contentType) {
			maps.Copy(meta, probe.Probe(content))
		}
	}
	return meta
}

// setIf stores a key only when the value is non-empty.
func setIf(meta map[string]string, key string, value string) {
	if value != "" {
		meta[key] = value
	}
}

// dateOnly trims an ISO timestamp to its date, leaving other strings unchanged.
func dateOnly(value string) string {
	if date, _, found := strings.Cut(value, "T"); found {
		return date
	}
	return value
}
