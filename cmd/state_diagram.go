package cmd

import (
	"fmt"

	"github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
	"github.com/AlexanderGrooff/mermaid-ascii/pkg/state"
	"github.com/elliotchance/orderedmap/v2"
)

type StateDiagramWrapper struct {
	parsed *state.StateDiagram
}

func (sd *StateDiagramWrapper) Parse(input string) error {
	parsed, err := state.Parse(input)
	if err != nil {
		return err
	}
	sd.parsed = parsed
	return nil
}

func (sd *StateDiagramWrapper) Render(config *diagram.Config) (string, error) {
	if sd.parsed == nil {
		return "", fmt.Errorf("state diagram not parsed: call Parse() before Render()")
	}
	if config == nil {
		config = diagram.DefaultConfig()
	}
	properties := stateToGraphProperties(sd.parsed, config)
	return drawMap(properties), nil
}

func (sd *StateDiagramWrapper) Type() string {
	return "state"
}

func stateToGraphProperties(sd *state.StateDiagram, config *diagram.Config) *graphProperties {
	data := orderedmap.NewOrderedMap[string, []textEdge]()
	nodeSpecs := make(map[string]graphNodeSpec)
	styleClasses := make(map[string]styleClass)

	// Build set of composite state IDs (these become subgraphs, not nodes)
	compositeIDs := make(map[string]bool)
	for _, cs := range sd.CompositeStates {
		compositeIDs[cs.ID] = true
	}

	// Add states in insertion order, skipping composite state IDs
	for _, id := range sd.StateOrder {
		if compositeIDs[id] {
			continue
		}
		s := sd.States[id]
		label := stateLabel(s)
		nodeSpecs[id] = graphNodeSpec{
			label:           newGraphLabel(label),
			labelIsExplicit: true,
		}
		data.Set(id, []textEdge{})
	}

	// Build composite member lookup for transition redirection
	compositeMembers := make(map[string][]string)
	for _, cs := range sd.CompositeStates {
		compositeMembers[cs.ID] = cs.Members
	}

	// Add transitions as edges, redirecting composite state references
	for _, t := range sd.Transitions {
		from, to := t.From, t.To

		// Redirect transitions targeting a composite to its first member
		if members, ok := compositeMembers[to]; ok && len(members) > 0 {
			to = members[0]
		}
		// Redirect transitions from a composite to its last member
		if members, ok := compositeMembers[from]; ok && len(members) > 0 {
			from = members[len(members)-1]
		}

		fromSpec := nodeSpecs[from]
		toSpec := nodeSpecs[to]
		edge := textEdge{
			parent: textNode{name: from, label: fromSpec.label, hasLabel: true},
			child:  textNode{name: to, label: toSpec.label, hasLabel: true},
			label:  t.Label,
		}
		if children, ok := data.Get(from); ok {
			data.Set(from, append(children, edge))
		}
	}

	// Convert composite states to textSubgraphs
	compositeMap := make(map[string]*textSubgraph)
	var subgraphs []*textSubgraph

	for _, cs := range sd.CompositeStates {
		sg := &textSubgraph{
			id:       cs.ID,
			name:     cs.Label,
			label:    newGraphLabel(cs.Label),
			nodes:    cs.Members,
			children: []*textSubgraph{},
		}
		compositeMap[cs.ID] = sg
		subgraphs = append(subgraphs, sg)
	}

	// Wire parent-child relationships between nested composites
	for _, cs := range sd.CompositeStates {
		sg := compositeMap[cs.ID]
		for _, memberID := range cs.Members {
			if childSg, ok := compositeMap[memberID]; ok {
				childSg.parent = sg
				sg.children = append(sg.children, childSg)
			}
		}
	}

	// Map direction: stateDiagram defaults to TB, graph uses "TD" for top-down
	graphDir := "TD"
	if sd.Direction == "LR" {
		graphDir = "LR"
	}

	styleType := config.StyleType
	if styleType == "" {
		styleType = "cli"
	}

	return &graphProperties{
		data:             data,
		nodeSpecs:        nodeSpecs,
		styleClasses:     &styleClasses,
		boxBorderPadding: config.BoxBorderPadding,
		graphDirection:   graphDir,
		styleType:        styleType,
		paddingX:         config.PaddingBetweenX,
		paddingY:         config.PaddingBetweenY,
		subgraphs:        subgraphs,
		useAscii:         config.UseAscii,
	}
}

func stateLabel(s *state.State) string {
	if s.Type == state.StateStart || s.Type == state.StateEnd {
		return "(*)"
	}
	return s.Label
}
