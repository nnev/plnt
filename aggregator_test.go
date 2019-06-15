package plnt

import (
	"strings"
	"testing"
)

func TestMakeAbsolute(t *testing.T) {
	const excerpt = `
<p>if required — which can contain fewer than 256 integers:</p>\n\n<p><img src="/turbopfor/ondisk.svgo.svg"/></p>\n\n<p>SIMD bitpacking</p>
`
	const baseURL = "https://michael.stapelberg.ch/"
	got, err := makeAbsolute(excerpt, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(excerpt, `src="/turbopfor`, `src="`+baseURL+`turbopfor`)
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got != want {
		t.Errorf("makeAbsolute:\ngot : %q\nwant: %q", got, want)
	}
}
