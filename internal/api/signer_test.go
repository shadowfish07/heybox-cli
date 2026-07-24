package api

import "testing"

func TestSignMatchesWebClientVectors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		path      string
		timestamp int64
		nonce     string
		want      string
	}{
		{
			name:      "canonical path",
			path:      "/bbs/app/api/general/search/v1",
			timestamp: 1_700_000_000,
			nonce:     "0123456789ABCDEF0123456789ABCDEF",
			want:      "RM29210",
		},
		{
			name:      "normalizes slashes",
			path:      "bbs/app/api/general/search/v1/",
			timestamp: 1_784_911_948,
			nonce:     "ABCDEF0123456789ABCDEF0123456789",
			want:      "AMQ2C30",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := sign(test.path, test.timestamp, test.nonce); got != test.want {
				t.Fatalf("sign() = %q, want %q", got, test.want)
			}
		})
	}
}
