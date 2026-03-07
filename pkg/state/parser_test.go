package state

import (
	"testing"
)

func TestIsStateDiagram(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"stateDiagram-v2", "stateDiagram-v2\n    [*] --> s1", true},
		{"stateDiagram", "stateDiagram\n    [*] --> s1", true},
		{"graph", "graph LR\n    A-->B", false},
		{"sequence", "sequenceDiagram\n    A->>B: Hi", false},
		{"empty", "", false},
		{"comment before keyword", "%% comment\nstateDiagram-v2\n    s1", true},
		{"only comments", "%% just a comment", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStateDiagram(tt.input); got != tt.want {
				t.Errorf("IsStateDiagram() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantStates      int
		wantTransitions int
		wantDirection   string
		wantErr         bool
	}{
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "missing keyword",
			input:   "s1 --> s2",
			wantErr: true,
		},
		{
			name:            "simple transition",
			input:           "stateDiagram-v2\n    s1 --> s2",
			wantStates:      2,
			wantTransitions: 1,
			wantDirection:   "TB",
		},
		{
			name:            "start and end pseudostates",
			input:           "stateDiagram-v2\n    [*] --> s1\n    s1 --> [*]",
			wantStates:      3,
			wantTransitions: 2,
			wantDirection:   "TB",
		},
		{
			name:            "transition with label",
			input:           "stateDiagram-v2\n    s1 --> s2 : trigger",
			wantStates:      2,
			wantTransitions: 1,
			wantDirection:   "TB",
		},
		{
			name:            "direction LR",
			input:           "stateDiagram-v2\n    direction LR\n    s1 --> s2",
			wantStates:      2,
			wantTransitions: 1,
			wantDirection:   "LR",
		},
		{
			name:            "direction TD normalized to TB",
			input:           "stateDiagram-v2\n    direction TD\n    s1 --> s2",
			wantStates:      2,
			wantTransitions: 1,
			wantDirection:   "TB",
		},
		{
			name:            "labeled state",
			input:           "stateDiagram-v2\n    state \"Moving\" as s1\n    [*] --> s1",
			wantStates:      2,
			wantTransitions: 1,
			wantDirection:   "TB",
		},
		{
			name:            "bare state",
			input:           "stateDiagram-v2\n    Idle",
			wantStates:      1,
			wantTransitions: 0,
			wantDirection:   "TB",
		},
		{
			name:    "unclosed composite",
			input:   "stateDiagram-v2\n    state comp {\n        s1 --> s2",
			wantErr: true,
		},
		{
			name:    "unexpected close brace",
			input:   "stateDiagram-v2\n    }",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() unexpected error: %v", err)
			}
			if len(sd.States) != tt.wantStates {
				t.Errorf("got %d states, want %d", len(sd.States), tt.wantStates)
			}
			if len(sd.Transitions) != tt.wantTransitions {
				t.Errorf("got %d transitions, want %d", len(sd.Transitions), tt.wantTransitions)
			}
			if sd.Direction != tt.wantDirection {
				t.Errorf("got direction %q, want %q", sd.Direction, tt.wantDirection)
			}
		})
	}
}

func TestStartEndPseudostates(t *testing.T) {
	sd, err := Parse("stateDiagram-v2\n    [*] --> s1\n    s1 --> [*]")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	start, ok := sd.States["__start__"]
	if !ok {
		t.Fatal("expected __start__ state")
	}
	if start.Type != StateStart {
		t.Errorf("start state type = %v, want StateStart", start.Type)
	}

	end, ok := sd.States["__end__"]
	if !ok {
		t.Fatal("expected __end__ state")
	}
	if end.Type != StateEnd {
		t.Errorf("end state type = %v, want StateEnd", end.Type)
	}
}

func TestScopedPseudostates(t *testing.T) {
	input := "stateDiagram-v2\n    [*] --> s1\n    state comp {\n        [*] --> inner\n    }"
	sd, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if _, ok := sd.States["__start__"]; !ok {
		t.Error("expected outer __start__")
	}
	if _, ok := sd.States["comp___start__"]; !ok {
		t.Error("expected inner comp___start__")
	}
}

func TestDistinctScopedPseudostatesAcrossComposites(t *testing.T) {
	input := `stateDiagram-v2
    state first {
        [*] --> a
        a --> [*]
    }
    state second {
        [*] --> b
        b --> [*]
    }`
	sd, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	for _, id := range []string{
		"first___start__",
		"first___end__",
		"second___start__",
		"second___end__",
	} {
		if _, ok := sd.States[id]; !ok {
			t.Errorf("expected state %q", id)
		}
	}

	if _, ok := sd.States["__start__"]; ok {
		t.Error("did not expect global __start__ without a top-level pseudostate")
	}
	if _, ok := sd.States["__end__"]; ok {
		t.Error("did not expect global __end__ without a top-level pseudostate")
	}
}

func TestNestedCompositePseudostatesRemainDistinct(t *testing.T) {
	input := `stateDiagram-v2
    state outer {
        [*] --> s1
        s1 --> [*]
        state inner {
            [*] --> s2
            s2 --> [*]
        }
    }`
	sd, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	for _, id := range []string{
		"outer___start__",
		"outer___end__",
		"inner___start__",
		"inner___end__",
	} {
		if _, ok := sd.States[id]; !ok {
			t.Errorf("expected state %q", id)
		}
	}
}

func TestCompositeStateMembers(t *testing.T) {
	input := "stateDiagram-v2\n    state \"Main\" as comp {\n        s1 --> s2\n        s2 --> s3\n    }"
	sd, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if len(sd.CompositeStates) != 1 {
		t.Fatalf("got %d composites, want 1", len(sd.CompositeStates))
	}

	cs := sd.CompositeStates[0]
	if cs.ID != "comp" {
		t.Errorf("composite ID = %q, want %q", cs.ID, "comp")
	}
	if cs.Label != "Main" {
		t.Errorf("composite label = %q, want %q", cs.Label, "Main")
	}
	if len(cs.Members) != 3 {
		t.Errorf("got %d members, want 3 (s1, s2, s3)", len(cs.Members))
	}
}

func TestLabeledState(t *testing.T) {
	input := "stateDiagram-v2\n    state \"Moving Forward\" as s1\n    s1 --> s2"
	sd, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	s1 := sd.States["s1"]
	if s1 == nil {
		t.Fatal("expected state s1")
	}
	if s1.Label != "Moving Forward" {
		t.Errorf("state label = %q, want %q", s1.Label, "Moving Forward")
	}
}
