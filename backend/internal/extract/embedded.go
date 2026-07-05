package extract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
)

// coreProperties is the docProps/core.xml shape of an office document's built-in metadata.
type coreProperties struct {
	Creator  string `xml:"creator"`
	Title    string `xml:"title"`
	Subject  string `xml:"subject"`
	Created  string `xml:"created"`
	Modified string `xml:"modified"`
}

// embeddedMeta returns best-effort metadata read from the file's own bytes: EXIF for images, core
// properties for office documents. It never fails; missing or unreadable metadata yields no keys.
func embeddedMeta(content []byte, contentType string) map[string]string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return imageEXIF(content)
	case isOffice(contentType):
		return officeCoreProps(content)
	default:
		return map[string]string{}
	}
}

// imageEXIF pulls capture time, GPS, and camera from an image's EXIF, when present.
func imageEXIF(content []byte) map[string]string {
	meta := map[string]string{}
	decoded, err := exif.Decode(bytes.NewReader(content))
	if err != nil {
		return meta
	}
	if taken, err := decoded.DateTime(); err == nil {
		meta["captured"] = taken.Format("2006-01-02")
	}
	if lat, long, err := decoded.LatLong(); err == nil {
		meta["gps"] = fmt.Sprintf("%.6f,%.6f", lat, long)
	}
	setIf(meta, "camera_make", exifString(decoded, exif.Make))
	setIf(meta, "camera_model", exifString(decoded, exif.Model))
	return meta
}

// exifString reads one EXIF string tag, returning "" when it is absent.
func exifString(decoded *exif.Exif, name exif.FieldName) string {
	tag, err := decoded.Get(name)
	if err != nil {
		return ""
	}
	value, err := tag.StringVal()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// officeCoreProps pulls the author, title, and dates from an office document's core properties.
func officeCoreProps(content []byte) map[string]string {
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
	if err := xml.NewDecoder(rc).Decode(&props); err != nil {
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
