package stompbox

import "testing"

func TestQuoteIfNeeded(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"foo", "foo"},
		{"foo bar", `"foo bar"`},
		{"tab\tsep", `"tab\tsep"`},
		{`he"llo`, `"he\"llo"`},
		{"  spaced  ", `"  spaced  "`}, // quoting decision is based on content, not trimming
	}

	for _, tc := range cases {
		got := quoteIfNeeded(tc.in)
		if got != tc.want {
			t.Fatalf("quoteIfNeeded(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestFirstProtocolError(t *testing.T) {
	t.Run("no error", func(t *testing.T) {
		resp := "Ok\r\n"
		if err := firstProtocolError(resp); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("error even if Ok later", func(t *testing.T) {
		resp := "Error something bad happened\r\nOk\r\n"
		if err := firstProtocolError(resp); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("whitespace tolerated", func(t *testing.T) {
		resp := "  Error   nope  \r\n  Ok  \r\n"
		if err := firstProtocolError(resp); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
