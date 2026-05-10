package eval

import (
	"sort"

	"github.com/divinity/divinity/internal/model"
)

func Score(result *model.AgentResult) {
	score := 0

	if result.Status == "succeeded" {
		score += 50
		result.EvaluationNotes = append(result.EvaluationNotes, "agent command completed successfully")
	} else {
		score -= 50
		result.EvaluationNotes = append(result.EvaluationNotes, "agent command failed")
	}

	passed := 0
	for _, check := range result.Validation {
		if check.ExitCode == 0 {
			passed++
		}
	}
	if len(result.Validation) > 0 {
		score += passed * 20
		result.EvaluationNotes = append(result.EvaluationNotes, "validation checks contribute to score")
	}

	if result.FilesChanged > 0 {
		score += 10
		result.EvaluationNotes = append(result.EvaluationNotes, "candidate produced a diff")
	} else {
		score -= 10
		result.EvaluationNotes = append(result.EvaluationNotes, "candidate did not change files")
	}

	if result.LinesAdded+result.LinesDeleted > 1200 {
		score -= 10
		result.EvaluationNotes = append(result.EvaluationNotes, "large diff penalized for review risk")
	}

	result.Score = score
}

func Recommend(results []model.AgentResult) string {
	if len(results) == 0 {
		return ""
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].DurationMillis < results[j].DurationMillis
		}
		return results[i].Score > results[j].Score
	})
	return results[0].Name
}
