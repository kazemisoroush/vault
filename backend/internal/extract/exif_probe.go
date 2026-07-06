package extract

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
)

// exifProbe reads capture time, GPS, and camera from an image's EXIF.
type exifProbe struct{}

// Supports reports whether the content is an image.
func (exifProbe) Supports(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

// Probe returns the image's EXIF metadata, recovering if the parser panics.
func (exifProbe) Probe(content []byte) (meta map[string]string) {
	meta = map[string]string{}
	defer func() {
		if recover() != nil {
			meta = map[string]string{}
		}
	}()
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
