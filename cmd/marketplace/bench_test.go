package main

import (
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/identity"
)

func TestChooseTakesTheMostExpensiveWithinBudget(t *testing.T) {
	results := []benchResult{
		{Params: identity.Params{Memory: 19456, Time: 2}, Median: 60 * time.Millisecond},
		{Params: identity.Params{Memory: 47104, Time: 2}, Median: 150 * time.Millisecond},
		{Params: identity.Params{Memory: 65536, Time: 3}, Median: 320 * time.Millisecond},
	}
	got := choose(results, 250*time.Millisecond)
	if got.Memory != 47104 || got.Time != 2 {
		t.Fatalf("choose = %+v, want m=47104 t=2", got)
	}
}

func TestChooseFallsBackToTheFloor(t *testing.T) {
	got := choose([]benchResult{{Params: identity.Params{Memory: 65536, Time: 3}, Median: time.Second}}, 250*time.Millisecond)
	if got.Memory != identity.Floor.Memory || got.Time != identity.Floor.Time {
		t.Fatalf("choose = %+v, want the OWASP floor", got)
	}
}
