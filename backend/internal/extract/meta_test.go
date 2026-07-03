package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMeta(t *testing.T) {
	// Arrange
	tests := []struct {
		name    string
		reply   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "plain json object",
			reply: `{"vendor":"Shell","amount":"52.30","date":"2026-06-01"}`,
			want:  map[string]string{"vendor": "Shell", "amount": "52.30", "date": "2026-06-01"},
		},
		{
			name:  "json wrapped in prose and fences",
			reply: "Here is the metadata:\n```json\n{\"airline\":\"THY\",\"pnr\":\"AB12CD\"}\n```",
			want:  map[string]string{"airline": "THY", "pnr": "AB12CD"},
		},
		{
			name:    "no json object",
			reply:   "I could not read the file.",
			wantErr: true,
		},
		{
			name:    "malformed json",
			reply:   `{"vendor": }`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, err := parseMeta(tc.reply)

			// Assert
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
