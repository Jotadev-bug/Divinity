package config

import "testing"

func TestSelectAgents(t *testing.T) {
	cfg := Config{
		Agents: []AgentConfig{
			{Name: "a"},
			{Name: "b"},
		},
	}

	selected, err := cfg.SelectAgents([]string{"b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 || selected[0].Name != "b" {
		t.Fatalf("expected b, got %#v", selected)
	}
}

func TestSelectAgentsRejectsUnknown(t *testing.T) {
	cfg := Config{Agents: []AgentConfig{{Name: "a"}}}
	if _, err := cfg.SelectAgents([]string{"missing"}); err == nil {
		t.Fatal("expected missing agent error")
	}
}
