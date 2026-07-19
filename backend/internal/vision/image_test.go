package vision

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestShrinkImageLeavesASmallImageAlone(t *testing.T) {
	// A small PNG is under the byte limit, so shrinkImage declines and the caller sends the original.
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 16, 16))))
	_, ok := shrinkImage(buf.Bytes())
	assert.False(t, ok)
}
