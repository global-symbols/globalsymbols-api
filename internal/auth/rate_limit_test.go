package auth

import "testing"

func TestEffectiveRateLimit(t *testing.T) {
	def := 100
	zero := 0
	high := 500

	if got := EffectiveRateLimit(def, nil); got != def {
		t.Fatalf("nil override: got %d want %d", got, def)
	}
	if got := EffectiveRateLimit(def, &zero); got != 0 {
		t.Fatalf("zero override (unlimited): got %d want 0", got)
	}
	if got := EffectiveRateLimit(def, &high); got != high {
		t.Fatalf("high override: got %d want %d", got, high)
	}
}
