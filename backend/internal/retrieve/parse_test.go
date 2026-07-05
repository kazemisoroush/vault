package retrieve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAnswer(t *testing.T) {
	tests := []struct {
		name       string
		reply      string
		wantText   string
		wantIDs    []string
		wantErr    bool
	}{
		{name: "answer and ids", reply: `{"answer":"RA3495037","ids":["a","b"]}`, wantText: "RA3495037", wantIDs: []string{"a", "b"}},
		{name: "empty answer", reply: `{"answer":"","ids":["x"]}`, wantText: "", wantIDs: []string{"x"}},
		{name: "no match", reply: `{"answer":"","ids":[]}`, wantText: "", wantIDs: []string{}},
		{name: "with surrounding text", reply: "Here: {\"answer\":\"hi\",\"ids\":[\"z\"]} done", wantText: "hi", wantIDs: []string{"z"}},
		{name: "no object", reply: `nothing here`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, err := parseAnswer(tc.reply)

			// Assert
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantText, got.Text)
			assert.Equal(t, tc.wantIDs, got.IDs)
		})
	}
}
