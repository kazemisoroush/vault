package vision

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/gif" // register the GIF decoder for image.Decode
	"image/jpeg"
	_ "image/png" // register the PNG decoder for image.Decode
	"net/http"
	"strings"

	_ "github.com/gen2brain/heic" // register the HEIC decoder (cgo-free, via wazero) for image.Decode
	xdraw "golang.org/x/image/draw"

	"github.com/kazemisoroush/vault/backend/internal/llm"
)

// maxImageEdgePixels is the longest side the model keeps before it downscales anyway, so a larger
// image is scaled to fit. maxImagePixels caps the pixels of a single decoded frame, so a compression
// bomb (a tiny file that decodes to a huge bitmap) cannot exhaust the function's memory. The cap is
// sized for the worst-case decode, the HEIC path, which holds two full-resolution copies at once
// (the wazero WASM linear memory plus the Go image copy) at roughly eight bytes per pixel, so 50M
// pixels is about 400 MB of transient bitmap, well under the ingest function's 1024 MB. A multi-frame
// HEIC sequence would decode every frame at once and slip past this single-frame cap, so toJPEG
// declines one outright.
const (
	maxImageEdgePixels = 1568
	maxImagePixels     = 50_000_000
)

// Supported reports whether a file needs vision transcription: an image or a PDF, which the
// Knowledge Base cannot index on its own.
func Supported(contentType string) bool {
	return strings.HasPrefix(contentType, "image/") || contentType == "application/pdf"
}

// DetectContentType returns the file's real media type. A browser often uploads a format it does not
// recognise, such as HEIC, as application/octet-stream, so when the declared type is generic the
// bytes are sniffed: first as an image (which recognises HEIC by its magic), then by content.
func DetectContentType(declared string, content []byte) string {
	if declared != "" && declared != "application/octet-stream" {
		return declared
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(content)); err == nil && format != "" {
		return "image/" + format
	}
	return http.DetectContentType(content)
}

// fileBlock returns the model content part for a file: an image (normalised to JPEG) or a PDF.
func fileBlock(content []byte, contentType string) llm.Part {
	if strings.HasPrefix(contentType, "image/") {
		return imageBlock(content)
	}
	return llm.Document(content)
}

// imageBlock returns the model content part for an image, decoded and re-encoded as JPEG so a format
// the model cannot read (HEIC) becomes one it can, and an oversized image is downscaled to fit the
// per-image limit. If the bytes cannot be decoded, the original is sent for the model to try.
func imageBlock(content []byte) llm.Part {
	if jpg, ok := toJPEG(content); ok {
		return llm.Image("image/jpeg", jpg)
	}
	return llm.Image(http.DetectContentType(content), content)
}

// toJPEG decodes any registered image format, including HEIC, and re-encodes it as JPEG under the
// model's per-image byte limit, downscaling an oversized one. It returns ok=false when the image is
// larger than the pixel cap or cannot be decoded, so the caller sends the original.
func toJPEG(content []byte) ([]byte, bool) {
	config, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return nil, false
	}
	// DecodeConfig reports one frame's size, but the HEIC decoder decodes every frame of a sequence
	// at once, so a small file with many frames slips past the single-frame cap above. Decline a
	// sequence rather than risk exhausting the function's memory; the model only reads one frame.
	if format == "heic" && heicHasMovieBox(content) {
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

// heicHasMovieBox reports whether the HEIC bytes carry a top-level moov box, which is what marks an
// image sequence. The decoder takes its all-frames path on exactly those files, so this walk mirrors
// the decoder's own top-level box parsing (a still image has a meta box and no moov). It reads only
// the box headers, never a payload, and stops on the first malformed length so it cannot loop or run
// off the end.
func heicHasMovieBox(content []byte) bool {
	for off := 0; off+8 <= len(content); {
		size := int(binary.BigEndian.Uint32(content[off : off+4]))
		if string(content[off+4:off+8]) == "moov" {
			return true
		}
		hdr := 8
		switch size {
		case 1:
			if off+16 > len(content) {
				return false
			}
			// A 64-bit size larger than the content cannot be valid, and converting it to int would
			// overflow and drive the walk out of bounds, so stop rather than trust it.
			large := binary.BigEndian.Uint64(content[off+8 : off+16])
			if large > uint64(len(content)) {
				return false
			}
			size = int(large)
			hdr = 16
		case 0:
			size = len(content) - off
		}
		if size < hdr || off+size > len(content) {
			return false
		}
		off += size
	}
	return false
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
