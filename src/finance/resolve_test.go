package finance

import "testing"

func TestResolveSymbol(t *testing.T) {
	got, err := ResolveSymbol("FR0013412269")
	if err != nil || got != "PANX.PA" {
		t.Fatalf("got %q, %v; want PANX.PA", got, err)
	}
}
