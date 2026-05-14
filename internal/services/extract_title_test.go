package services

import "testing"

func TestExtractTitle_DecodesEntities(t *testing.T) {
	nm := &NoteManager{}
	cases := []struct {
		html, want string
	}{
		{`<html><title>It&#x27;s FOSS</title></html>`, "It's FOSS"},
		{`<html><title>Foo &amp; Bar</title></html>`, "Foo & Bar"},
		{`<html><title>Tom &quot;the Tank&quot;</title></html>`, `Tom "the Tank"`},
		{`<html><title>Plain Title</title></html>`, "Plain Title"},
	}
	for _, tc := range cases {
		got := nm.extractTitle(tc.html, "example.com")
		if got != tc.want {
			t.Errorf("extractTitle(%q) = %q, want %q", tc.html, got, tc.want)
		}
	}
}

func TestExtractTitle_FallsBackToHost(t *testing.T) {
	nm := &NoteManager{}
	got := nm.extractTitle(`<html>no title here</html>`, "example.com")
	if got != "example.com" {
		t.Errorf("fallback = %q, want example.com", got)
	}
}
