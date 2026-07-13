package extract

import (
	"bytes"
	"image"
	"image/jpeg"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noiseJPEG builds a width by height JPEG of random pixels. Noise compresses poorly, so a large one
// exceeds the size limit, which lets a test build a genuinely oversized image.
func noiseJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	source := rand.New(rand.NewSource(1))
	_, _ = source.Read(img.Pix)
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}))
	return buf.Bytes()
}

func TestShrinkImageLeavesSmallImagesAlone(t *testing.T) {
	// Arrange: an image well under the limit.
	small := noiseJPEG(t, 200, 200)
	require.LessOrEqual(t, len(small), maxImageBytes)

	// Act
	_, ok := shrinkImage(small)

	// Assert: nothing to do, so the caller sends the original.
	assert.False(t, ok)
}

func TestShrinkImageDownsizesAnOversizedImage(t *testing.T) {
	// Arrange: a large image whose bytes exceed the limit, like a phone photo.
	big := noiseJPEG(t, 4000, 3000)
	require.Greater(t, len(big), maxImageBytes)

	// Act
	out, ok := shrinkImage(big)

	// Assert: it shrank, still decodes, fits the byte budget, and is capped to the max edge.
	require.True(t, ok)
	assert.LessOrEqual(t, len(out), maxImageBytes)
	decoded, _, err := image.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	bounds := decoded.Bounds()
	assert.LessOrEqual(t, bounds.Dx(), maxImageEdge)
	assert.LessOrEqual(t, bounds.Dy(), maxImageEdge)
}

func TestShrinkImageIgnoresUndecodableBytes(t *testing.T) {
	// Arrange: bytes over the limit that are not a decodable image (for example a TIFF or HEIC).
	notAnImage := bytes.Repeat([]byte("not an image"), 400_000)
	require.Greater(t, len(notAnImage), maxImageBytes)

	// Act
	_, ok := shrinkImage(notAnImage)

	// Assert: cannot decode, so the caller sends the original unchanged.
	assert.False(t, ok)
}
