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

// Image limits keep an oversized image within the model's per-image size cap. maxImageBytes is the
// raw-byte budget that keeps the base64 payload under the roughly 5 MB per-image limit, since base64
// inflates bytes by about 4/3. maxImageEdge is the longest side the model keeps before it downscales
// anyway.
const (
	maxImageBytes = 3_600_000
	maxImageEdge  = 1568
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
// It returns ok=false when the image is already small enough or cannot be decoded, so the caller
// sends the original.
func shrinkImage(content []byte) ([]byte, bool) {
	if len(content) <= maxImageBytes {
		return nil, false
	}
	source, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, false
	}

	bounds := scaledBounds(source.Bounds(), maxImageEdge)
	scaled := image.NewRGBA(bounds)
	xdraw.CatmullRom.Scale(scaled, bounds, source, source.Bounds(), xdraw.Src, nil)

	return encodeUnderLimit(scaled)
}

// scaledBounds returns the destination rectangle for an image whose longest edge is capped at max,
// keeping the aspect ratio. An image already within the cap keeps its size.
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
	return image.Rect(0, 0, int(float64(width)*scale), int(float64(height)*scale))
}

// encodeUnderLimit encodes the image as JPEG, dropping the quality until the result fits the raw
// byte budget so the base64 payload stays under the limit. It returns ok=false only if nothing
// could be encoded, which does not happen for a decoded image capped to maxImageEdge.
func encodeUnderLimit(img image.Image) ([]byte, bool) {
	var best []byte
	for quality := 85; quality >= 40; quality -= 15 {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			continue
		}
		best = buf.Bytes()
		if len(best) <= maxImageBytes {
			return best, true
		}
	}
	return best, best != nil
}
