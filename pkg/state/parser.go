package state

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
)

var (
	// direction LR, direction TB, direction TD
	directionRegex = regexp.MustCompile(`^\s*direction\s+(LR|RL|TB|TD|BT)\s*$`)

	// s1 --> s2 or s1 --> s2 : label
	transitionRegex = regexp.MustCompile(`^\s*(\[\*\]|[\w]+)\s*-->\s*(\[\*\]|[\w]+)\s*(?::\s*(.+?))?\s*$`)

	// state "Label" as id
	stateDeclRegex = regexp.MustCompile(`^\s*state\s+"([^"]+)"\s+as\s+(\w+)\s*$`)

	// bare state identifier
	bareStateRegex = regexp.MustCompile(`^\s*(\w+)\s*$`)

	// state "Label" as id {  (composite open with label and alias)
	compositeOpenLabeledRegex = regexp.MustCompile(`^\s*state\s+"([^"]+)"\s+as\s+(\w+)\s*\{\s*$`)

	// state id {  (composite open, simple)
	compositeOpenSimpleRegex = regexp.MustCompile(`^\s*state\s+(\w+)\s*\{\s*$`)

	// closing brace
	closeBraceRegex = regexp.MustCompile(`^\s*\}\s*$`)
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

	var compositeStack []*CompositeState

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Closing brace — pop composite stack
		if closeBraceRegex.MatchString(trimmed) {
			if len(compositeStack) == 0 {
				return nil, fmt.Errorf("line %d: unexpected '}'", i+2)
			}
			compositeStack = compositeStack[:len(compositeStack)-1]
			continue
		}

		// Composite state open with label: state "Label" as id {
		if match := compositeOpenLabeledRegex.FindStringSubmatch(trimmed); match != nil {
			cs := &CompositeState{ID: match[2], Label: match[1], Members: []string{}}
			sd.CompositeStates = append(sd.CompositeStates, cs)
			sd.ensureState(cs.ID, StateNormal)
			sd.States[cs.ID].Label = cs.Label
			if len(compositeStack) > 0 {
				parent := compositeStack[len(compositeStack)-1]
				sd.addToComposite(parent, cs.ID)
			}
			compositeStack = append(compositeStack, cs)
			continue
		}

		// Composite state open simple: state id {
		if match := compositeOpenSimpleRegex.FindStringSubmatch(trimmed); match != nil {
			cs := &CompositeState{ID: match[1], Label: match[1], Members: []string{}}
			sd.CompositeStates = append(sd.CompositeStates, cs)
			sd.ensureState(cs.ID, StateNormal)
			if len(compositeStack) > 0 {
				parent := compositeStack[len(compositeStack)-1]
				sd.addToComposite(parent, cs.ID)
			}
			compositeStack = append(compositeStack, cs)
			continue
		}

		// Track states before parsing so we can add new ones to composites
		statesBefore := len(sd.StateOrder)

		// Determine scope prefix for pseudostate IDs inside composites
		scope := ""
		if len(compositeStack) > 0 {
			scope = compositeStack[len(compositeStack)-1].ID + "_"
		}

		if err := sd.parseLine(trimmed, scope); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+2, err)
		}

		// Add newly created states to the active composite
		if len(compositeStack) > 0 {
			active := compositeStack[len(compositeStack)-1]
			for _, id := range sd.StateOrder[statesBefore:] {
				sd.addToComposite(active, id)
			}
		}
	}

	if len(compositeStack) > 0 {
		return nil, fmt.Errorf("unclosed composite state: %s", compositeStack[len(compositeStack)-1].ID)
	}

	return sd, nil
}

func (sd *StateDiagram) parseLine(line string, scope string) error {
	// Direction directive
	if match := directionRegex.FindStringSubmatch(line); match != nil {
		dir := match[1]
		if dir == "TD" {
			dir = "TB"
		}
		sd.Direction = dir
		return nil
	}

	// State declaration: state "Label" as id
	if match := stateDeclRegex.FindStringSubmatch(line); match != nil {
		label := match[1]
		id := match[2]
		sd.ensureState(id, StateNormal)
		sd.States[id].Label = label
		return nil
	}

	// Transition: A --> B or A --> B : label
	if match := transitionRegex.FindStringSubmatch(line); match != nil {
		fromRaw, toRaw, label := match[1], match[2], match[3]
		fromID, fromType := resolveStateID(fromRaw, true, scope)
		toID, toType := resolveStateID(toRaw, false, scope)
		sd.ensureState(fromID, fromType)
		sd.ensureState(toID, toType)
		sd.Transitions = append(sd.Transitions, &Transition{
			From:  fromID,
			To:    toID,
			Label: strings.TrimSpace(label),
		})
		return nil
	}

	// Bare state identifier
	if match := bareStateRegex.FindStringSubmatch(line); match != nil {
		sd.ensureState(match[1], StateNormal)
		return nil
	}

	return fmt.Errorf("unknown syntax: %q", line)
}

func resolveStateID(raw string, isSource bool, scope string) (string, StateType) {
	if raw == "[*]" {
		if isSource {
			return scope + "__start__", StateStart
		}
		return scope + "__end__", StateEnd
	}
	return raw, StateNormal
}

func (sd *StateDiagram) addToComposite(cs *CompositeState, stateID string) {
	for _, m := range cs.Members {
		if m == stateID {
			return
		}
	}
	cs.Members = append(cs.Members, stateID)
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
