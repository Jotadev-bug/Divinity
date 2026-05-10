package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/divinity/divinity/internal/model"
)

const DirName = ".divinity"

func EnsureLayout(root string) error {
	for _, dir := range []string{
		filepath.Join(root, DirName),
		filepath.Join(root, DirName, "runs"),
		filepath.Join(root, DirName, "worktrees"),
		filepath.Join(root, DirName, "logs"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func RunDir(root, runID string) string {
	return filepath.Join(root, DirName, "runs", runID)
}

func SaveRun(root string, run model.Run) error {
	dir := RunDir(root, run.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, "run.json"), data, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, DirName, "latest"), []byte(run.ID), 0644)
}

func ListRuns(root string) ([]model.Run, error) {
	base := filepath.Join(root, DirName, "runs")
	entries, err := os.ReadDir(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	runs := make([]model.Run, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		run, err := LoadRun(root, entry.Name())
		if err == nil {
			runs = append(runs, run)
		}
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	return runs, nil
}

func LoadRun(root, runID string) (model.Run, error) {
	if runID == "" {
		data, err := os.ReadFile(filepath.Join(root, DirName, "latest"))
		if err != nil {
			return model.Run{}, fmt.Errorf("no latest run found")
		}
		runID = string(data)
	}

	data, err := os.ReadFile(filepath.Join(RunDir(root, runID), "run.json"))
	if err != nil {
		return model.Run{}, err
	}

	var run model.Run
	if err := json.Unmarshal(data, &run); err != nil {
		return model.Run{}, err
	}
	return run, nil
}
