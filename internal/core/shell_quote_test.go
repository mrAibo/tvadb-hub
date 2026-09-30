package core

import "testing"

func TestQuoteShellArg(t *testing.T) {
	cases := map[string]string{
		"plain": "hello",
		"spaces": "hello world",
		"metacharacters": "hello; touch /sdcard/pwned",
		"single quote": "don't",
	}

	want := map[string]string{
		"plain": "'hello'",
		"spaces": "'hello world'",
		"metacharacters": "'hello; touch /sdcard/pwned'",
		"single quote": "'don'\"'\"'t'",
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if got := QuoteShellArg(input); got != want[name] {
				t.Fatalf("QuoteShellArg(%q) = %q, want %q", input, got, want[name])
			}
		})
	}
}
