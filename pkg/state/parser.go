package state

import (
	"fmt"
	"strings"

	"github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
)

type StateType int

const (
	StateNormal StateType = iota
	StateStart
	StateEnd
)

type State struct {
	ID    string
	Label string
	Type  StateType
}

type Transition struct {
	From  string
	To    string
	Label string
}

type CompositeState struct {
	ID      string
	Label   string
	Members []string
}

type StateDiagram struct {
	States          map[string]*State
	StateOrder      []string // insertion order for deterministic layout
	Transitions     []*Transition
	CompositeStates []*CompositeState
	Direction       string // "LR" or "TB", default "TB"
}

func IsStateDiagram(input string) bool {
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		return strings.HasPrefix(trimmed, "stateDiagram")
	}
	return false
}

func Parse(input string) (*StateDiagram, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}

	rawLines := diagram.SplitLines(input)
	lines := diagram.RemoveComments(rawLines)
	if len(lines) == 0 {
		return nil, fmt.Errorf("no content found")
	}

	first := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(first, "stateDiagram") {
		return nil, fmt.Errorf("expected \"stateDiagram\" or \"stateDiagram-v2\" keyword")
	}
	lines = lines[1:]

	sd := &StateDiagram{
		States:          make(map[string]*State),
		StateOrder:      []string{},
		Transitions:     []*Transition{},
		CompositeStates: []*CompositeState{},
		Direction:       "TB",
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if err := sd.parseLine(trimmed); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+2, err)
		}
	}

	return sd, nil
}

func (sd *StateDiagram) parseLine(line string) error {
	// Placeholder — will be implemented in commit 2
	return fmt.Errorf("unknown syntax: %q", line)
}

func (sd *StateDiagram) ensureState(id string, stateType StateType) {
	if _, exists := sd.States[id]; !exists {
		label := id
		if stateType == StateStart || stateType == StateEnd {
			label = "[*]"
		}
		sd.States[id] = &State{ID: id, Label: label, Type: stateType}
		sd.StateOrder = append(sd.StateOrder, id)
	}
}
