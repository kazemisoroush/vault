package retrieve

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIDs(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		want    []string
		wantErr bool
	}{
		{name: "bare array", reply: `["a","b"]`, want: []string{"a", "b"}},
		{name: "with surrounding text", reply: `Here you go: ["x"] done`, want: []string{"x"}},
		{name: "empty array", reply: `[]`, want: []string{}},
		{name: "no array", reply: `none found`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, err := parseIDs(tc.reply)

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
