package eval

import (
	"testing"

	"github.com/divinity/divinity/internal/model"
)

func TestRecommendHighestScore(t *testing.T) {
	winner := Recommend([]model.AgentResult{
		{Name: "slow", Score: 10, DurationMillis: 1000},
		{Name: "fast", Score: 20, DurationMillis: 500},
	})
	if winner != "fast" {
		t.Fatalf("expected fast, got %s", winner)
	}
}

func TestRecommendUsesDurationAsTieBreak(t *testing.T) {
	winner := Recommend([]model.AgentResult{
		{Name: "slow", Score: 20, DurationMillis: 1000},
		{Name: "fast", Score: 20, DurationMillis: 500},
	})
	if winner != "fast" {
		t.Fatalf("expected fast, got %s", winner)
	}
}
