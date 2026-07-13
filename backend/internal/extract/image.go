package extract

import (
	"bytes"
	"image"
	_ "image/gif" // register the GIF decoder for image.Decode
	"image/jpeg"
	_ "image/png" // register the PNG decoder for image.Decode

	xdraw "golang.org/x/image/draw"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// maxImageEdgePixels is the longest side the model keeps before it downscales anyway, so a larger image is
// scaled to fit. maxImagePixels caps how big an image we will decode, so a compression bomb (a tiny
// file that decodes to a huge bitmap) cannot exhaust the function's memory.
const (
	maxImageEdgePixels   = 1568
	maxImagePixels = 100_000_000
)

// imageBlock returns the model content part for an image, downscaling and re-encoding one that is
// too large so it stays within the per-image limit. The stored file keeps its original bytes; only
// this copy is shrunk.
func imageBlock(content []byte, contentType string) llm.Part {
	if shrunk, ok := shrinkImage(content); ok {
		return llm.Image("image/jpeg", shrunk)
	}
	return llm.Image(contentType, content)
}

// shrinkImage downscales an oversized image to fit the per-image limit and returns it as JPEG bytes.
// It returns ok=false when the image is already small enough, is larger than the pixel cap, or
// cannot be decoded, so the caller sends the original.
func shrinkImage(content []byte) ([]byte, bool) {
	if len(content) <= llm.MaxImageBytes {
		return nil, false
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return nil, false
	}
	source, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, false
	}

	bounds := scaledBounds(source.Bounds(), maxImageEdgePixels)
	scaled := image.NewRGBA(bounds)
	// JPEG has no alpha, so flatten any transparency onto white rather than the zero value (black).
	xdraw.Draw(scaled, bounds, image.White, image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(scaled, bounds, source, source.Bounds(), xdraw.Over, nil)

	return encodeUnderLimit(scaled)
}

// scaledBounds returns the destination rectangle for an image whose longest edge is capped at max,
// keeping the aspect ratio. An image already within the cap keeps its size, and each edge is at
// least one pixel so an extreme aspect ratio does not round to an empty image.
func scaledBounds(b image.Rectangle, max int) image.Rectangle {
	width, height := b.Dx(), b.Dy()
	longest := width
	if height > longest {
		longest = height
	}
	if longest <= max {
		return image.Rect(0, 0, width, height)
	}
	scale := float64(max) / float64(longest)
	return image.Rect(0, 0, atLeastOne(int(float64(width)*scale)), atLeastOne(int(float64(height)*scale)))
}

// atLeastOne clamps a scaled edge to a minimum of one pixel.
func atLeastOne(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// encodeUnderLimit encodes the image as JPEG, dropping the quality until the result fits the byte
// budget so the base64 payload stays under the limit. It returns ok=false only if nothing could be
// encoded, which does not happen for a decoded image capped to maxImageEdgePixels.
func encodeUnderLimit(img image.Image) ([]byte, bool) {
	var best []byte
	for quality := 85; quality >= 40; quality -= 15 {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			continue
		}
		best = buf.Bytes()
		if len(best) <= llm.MaxImageBytes {
			return best, true
		}
	}
	return best, best != nil
}
