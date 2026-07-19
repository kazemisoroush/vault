package vision

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// box builds one ISOBMFF box (a 32-bit size, a 4-char type, then the payload), the shape the HEIC
// container uses.
func box(typ string, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(b[0:4], uint32(8+len(payload)))
	copy(b[4:8], typ)
	copy(b[8:], payload)
	return b
}

func TestSupportedCoversImagesAndPDFs(t *testing.T) {
	assert.True(t, Supported("image/jpeg"))
	assert.True(t, Supported("image/png"))
	assert.True(t, Supported("application/pdf"))
	assert.False(t, Supported("text/plain"))
	assert.False(t, Supported("application/vnd.openxmlformats-officedocument.wordprocessingml.document"))
	assert.False(t, Supported("audio/m4a"))
}

func TestScaledBoundsCapsLongestEdgeKeepingAspect(t *testing.T) {
	// A 4000x2000 image scaled to a 1568 cap keeps its 2:1 aspect ratio.
	got := scaledBounds(image.Rect(0, 0, 4000, 2000), 1568)
	assert.Equal(t, 1568, got.Dx())
	assert.Equal(t, 784, got.Dy())

	// An image already within the cap is unchanged.
	small := scaledBounds(image.Rect(0, 0, 800, 600), 1568)
	assert.Equal(t, 800, small.Dx())
	assert.Equal(t, 600, small.Dy())
}

func TestScaledBoundsNeverRoundsToEmpty(t *testing.T) {
	// An extreme aspect ratio keeps each edge at least one pixel.
	got := scaledBounds(image.Rect(0, 0, 10000, 1), 1568)
	assert.GreaterOrEqual(t, got.Dx(), 1)
	assert.GreaterOrEqual(t, got.Dy(), 1)
}

func TestToJPEGNormalizesAPNG(t *testing.T) {
	// A PNG is decoded and re-encoded as JPEG, so the model always receives a format it can read.
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 32, 24))))

	jpg, ok := toJPEG(buf.Bytes())
	require.True(t, ok)
	_, format, err := image.DecodeConfig(bytes.NewReader(jpg))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
}

func TestToJPEGDeclinesNonImageBytes(t *testing.T) {
	_, ok := toJPEG([]byte("this is not an image"))
	assert.False(t, ok)
}

func TestHeicHasMovieBoxDetectsASequence(t *testing.T) {
	still := append(box("ftyp", []byte("heicmif1")), box("meta", []byte("meta payload"))...)
	still = append(still, box("mdat", []byte("image data"))...)
	assert.False(t, heicHasMovieBox(still), "a still image has no moov box")

	sequence := append(box("ftyp", []byte("msf1hevc")), box("moov", []byte("movie payload"))...)
	assert.True(t, heicHasMovieBox(sequence), "an image sequence carries a moov box")
}

func TestHeicHasMovieBoxHandlesMalformedBytesWithoutPanic(t *testing.T) {
	// A truncated header, a box shorter than its header, and a length running past the end must all
	// stop the walk rather than loop or read out of bounds.
	assert.False(t, heicHasMovieBox([]byte("ftyp")))
	assert.False(t, heicHasMovieBox([]byte{0, 0, 0, 4, 'f', 't', 'y', 'p'}))
	overrun := []byte{0, 0, 0, 255, 'f', 't', 'y', 'p', 'm', 'i', 'f', '1'}
	assert.False(t, heicHasMovieBox(overrun))
	assert.False(t, heicHasMovieBox(nil))
}

func TestDetectContentTypeCorrectsAMislabelledImage(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))))

	// A real declared type is trusted as-is.
	assert.Equal(t, "image/jpeg", DetectContentType("image/jpeg", buf.Bytes()))
	// A browser that uploads an image as octet-stream is corrected by sniffing the bytes.
	assert.Equal(t, "image/png", DetectContentType("application/octet-stream", buf.Bytes()))
	// Non-image bytes stay generic, so they are not routed to transcription.
	assert.False(t, strings.HasPrefix(DetectContentType("application/octet-stream", []byte("plain text")), "image/"))
}
