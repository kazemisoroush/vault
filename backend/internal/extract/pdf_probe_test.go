package extract

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// pdfWithInfo builds a minimal valid PDF whose /Info dictionary carries the given properties.
func pdfWithInfo(t *testing.T, author, title, created string) []byte {
	t.Helper()
	var buf bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}

	buf.WriteString("%PDF-1.4\n")
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	obj("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>")
	obj(fmt.Sprintf("<< /Author (%s) /Title (%s) /CreationDate (%s) >>", author, title, created))

	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(offsets)+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R /Info 4 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xrefOffset)
	return buf.Bytes()
}

func TestPDFProbeSupports(t *testing.T) {
	assert.True(t, pdfProbe{}.Supports("application/pdf"))
	assert.False(t, pdfProbe{}.Supports("image/jpeg"))
	assert.False(t, pdfProbe{}.Supports("text/plain"))
}

func TestPDFProbeReadsInfoDictionary(t *testing.T) {
	// Arrange
	content := pdfWithInfo(t, "Soroush Kazemi", "Quarterly Report", "D:20260601120000Z")

	// Act
	meta := embeddedMeta(content, "application/pdf")

	// Assert
	assert.Equal(t, "Soroush Kazemi", meta["author"])
	assert.Equal(t, "Quarterly Report", meta["title"])
	assert.Equal(t, "2026-06-01", meta["created"])
}

func TestPDFProbeNonPDFIsEmpty(t *testing.T) {
	// Act + Assert: garbage bytes never panic and yield no metadata.
	assert.Empty(t, pdfProbe{}.Probe([]byte("not a pdf at all")))
}

func TestPDFDate(t *testing.T) {
	assert.Equal(t, "2026-06-01", pdfDate("D:20260601120000Z"))
	assert.Equal(t, "", pdfDate("2026-06-01"))
	assert.Equal(t, "", pdfDate(""))
}
