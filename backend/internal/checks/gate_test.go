package checks_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazemisoroush/vault/backend/internal/checks"
)

// The gate enforces the north star: zero false greens. Every adversarial case here must fail
// closed. A failing case in this file is a product incident, not a flaky test.

func TestLocateFindsExactSpan(t *testing.T) {
	text := "The contract was executed on 14 February 2023.\nThe deposit of $40,000 was payable within seven days."

	start, end, ok := checks.Locate(text, "deposit of $40,000")

	require.True(t, ok)
	assert.Equal(t, "deposit of $40,000", text[start:end])
}

func TestLocateRejectsEmptySpan(t *testing.T) {
	_, _, ok := checks.Locate("some text", "")
	assert.False(t, ok, "an empty span must never gain offsets")
}

func TestLocateRejectsAbsentSpan(t *testing.T) {
	_, _, ok := checks.Locate("the tenant paid on time", "the tenant paid late")
	assert.False(t, ok)
}

func TestLocateRejectsUnicodeLookalikes(t *testing.T) {
	// The stored text uses a plain space and straight quote; the judge's span uses an NBSP and a
	// curly quote. Close enough for a human, not for the gate.
	text := `The deposit of $40,000 was "payable" within seven days.`

	cases := map[string]string{
		"nbsp instead of space":    "deposit of $40,000",
		"curly quotes":             "was “payable” within",
		"soft hyphen inserted":     "pay\u00adable",
		"different dash":           "40–000",
		"ligature fi":              "ﬁve",
		"trailing extra space":     "seven days. ",
		"case drift":               "The Deposit of $40,000",
		"ellipsis instead of dots": "days…",
	}
	for name, span := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, ok := checks.Locate(text, span)
			assert.False(t, ok, "lookalike span must fail closed")
		})
	}
}

func TestVerifyConfirmsExactOffsets(t *testing.T) {
	text := "The contract was executed on 14 February 2023."
	start, end, ok := checks.Locate(text, "executed on 14 February 2023")
	require.True(t, ok)

	assert.True(t, checks.Verify(text, "executed on 14 February 2023", start, end))
}

func TestVerifyRejectsOffByOne(t *testing.T) {
	text := "The deposit of $40,000 was payable within seven days."
	span := "deposit of $40,000"
	start, end, ok := checks.Locate(text, span)
	require.True(t, ok)

	assert.False(t, checks.Verify(text, span, start+1, end+1), "shifted offsets must fail")
	assert.False(t, checks.Verify(text, span, start, end+1), "stretched offsets must fail")
	assert.False(t, checks.Verify(text, span, start-1, end), "negative drift must fail")
}

func TestVerifyRejectsOutOfRangeOffsets(t *testing.T) {
	text := "short"

	assert.False(t, checks.Verify(text, "short", 0, 99), "end past the text must fail")
	assert.False(t, checks.Verify(text, "short", -1, 5), "negative start must fail")
	assert.False(t, checks.Verify(text, "", 0, 0), "empty span must fail")
	assert.False(t, checks.Verify(text, "tro", 3, 2), "inverted range must fail")
}

func TestVerifyRejectsSpanFromAnotherDocument(t *testing.T) {
	// The span is real prose, but it belongs to a different document than the offsets point into.
	contract := "The deposit of $40,000 was payable within seven days."
	email := "We acknowledge the delay and will respond in early March."
	start, end, ok := checks.Locate(email, "acknowledge the delay")
	require.True(t, ok)

	assert.False(t, checks.Verify(contract, "acknowledge the delay", start, end),
		"a span located in one document must not verify against another")
}
