package cmd

import "testing"

func TestSplitGraphLines(t *testing.T) {
	input := "graph LR\\nA[\"line1\\nline2\"] --> B\\nC --> D"

	got := splitGraphLines(input)
	want := []string{"graph LR", `A["line1\nline2"] --> B`, "C --> D"}

	if len(got) != len(want) {
		t.Fatalf("line count = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitGraphLinesKeepsLiteralNewlineInsideNodeLabel(t *testing.T) {
	input := "graph LR\nA[\"line1\nline2\"] --> B\nC --> D"

	got := splitGraphLines(input)
	want := []string{"graph LR", "A[\"line1\nline2\"] --> B", "C --> D"}

	if len(got) != len(want) {
		t.Fatalf("line count = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseNodeWithExplicitLabel(t *testing.T) {
	node := parseNode(`A["line1<br/>line2"]:::primary`)

	if node.name != "A" {
		t.Fatalf("name = %q, want %q", node.name, "A")
	}
	if node.styleClass != "primary" {
		t.Fatalf("styleClass = %q, want %q", node.styleClass, "primary")
	}
	if len(node.label.lines) != 2 {
		t.Fatalf("label lines = %d, want 2", len(node.label.lines))
	}
	if node.label.lines[0] != "line1" || node.label.lines[1] != "line2" {
		t.Fatalf("label lines = %#v, want [line1 line2]", node.label.lines)
	}
}

func TestMermaidFileToMapPreservesEscapedLabelNewlines(t *testing.T) {
	properties, err := mermaidFileToMap("graph LR\\nA[\"line1\\nline2\"] --> B", "cli")
	if err != nil {
		t.Fatalf("mermaidFileToMap() error = %v", err)
	}

	spec := properties.nodeSpecs["A"]
	if len(spec.label.lines) != 2 {
		t.Fatalf("label lines = %d, want 2", len(spec.label.lines))
	}
	if spec.label.lines[0] != "line1" || spec.label.lines[1] != "line2" {
		t.Fatalf("label lines = %#v, want [line1 line2]", spec.label.lines)
	}
}

func TestMermaidFileToMapPreservesLiteralLabelNewlines(t *testing.T) {
	properties, err := mermaidFileToMap("graph LR\nA[\"line1\nline2\"] --> B", "cli")
	if err != nil {
		t.Fatalf("mermaidFileToMap() error = %v", err)
	}

	spec := properties.nodeSpecs["A"]
	if len(spec.label.lines) != 2 {
		t.Fatalf("label lines = %d, want 2", len(spec.label.lines))
	}
	if spec.label.lines[0] != "line1" || spec.label.lines[1] != "line2" {
		t.Fatalf("label lines = %#v, want [line1 line2]", spec.label.lines)
	}
}

func TestParseSubgraphHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantID    string
		wantLabel string
	}{
		{
			name:      "plain title",
			header:    "Frontend",
			wantID:    "",
			wantLabel: "Frontend",
		},
		{
			name:      "explicit id and title",
			header:    "frontend [Frontend Services]",
			wantID:    "frontend",
			wantLabel: "Frontend Services",
		},
		{
			name:      "quoted title",
			header:    `frontend["Frontend Services"]`,
			wantID:    "frontend",
			wantLabel: "Frontend Services",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sg := parseSubgraphHeader(tt.header)
			if sg.id != tt.wantID {
				t.Fatalf("id = %q, want %q", sg.id, tt.wantID)
			}
			if sg.name != tt.wantLabel {
				t.Fatalf("name = %q, want %q", sg.name, tt.wantLabel)
			}
			if len(sg.label.lines) != 1 || sg.label.lines[0] != tt.wantLabel {
				t.Fatalf("label lines = %#v, want [%s]", sg.label.lines, tt.wantLabel)
			}
		})
	}
}

func TestMermaidFileToMapParsesSubgraphIDAndTitle(t *testing.T) {
	properties, err := mermaidFileToMap("graph LR\nsubgraph frontend [Frontend Services]\nA --> B\nend", "cli")
	if err != nil {
		t.Fatalf("mermaidFileToMap() error = %v", err)
	}

	if len(properties.subgraphs) != 1 {
		t.Fatalf("subgraphs = %d, want 1", len(properties.subgraphs))
	}

	sg := properties.subgraphs[0]
	if sg.id != "frontend" {
		t.Fatalf("id = %q, want %q", sg.id, "frontend")
	}
	if sg.name != "Frontend Services" {
		t.Fatalf("name = %q, want %q", sg.name, "Frontend Services")
	}
}

func TestMermaidFileToMapKeepsExplicitNodeLabelAcrossBareReferences(t *testing.T) {
	properties, err := mermaidFileToMap("graph TD\nA[\"Foo\"] --> B[\"Bar\"]\nB --> C[\"Baz\"]", "cli")
	if err != nil {
		t.Fatalf("mermaidFileToMap() error = %v", err)
	}

	spec := properties.nodeSpecs["B"]
	if len(spec.label.lines) != 1 || spec.label.lines[0] != "Bar" {
		t.Fatalf("label lines = %#v, want [Bar]", spec.label.lines)
	}
	if !spec.labelIsExplicit {
		t.Fatal("expected B label to remain explicit")
	}
}

func TestMermaidFileToMapUsesLatestExplicitLabel(t *testing.T) {
	properties, err := mermaidFileToMap("graph TD\nA[\"Old\"] --> B\nA[\"New\"] --> C", "cli")
	if err != nil {
		t.Fatalf("mermaidFileToMap() error = %v", err)
	}

	spec := properties.nodeSpecs["A"]
	if len(spec.label.lines) != 1 || spec.label.lines[0] != "New" {
		t.Fatalf("label lines = %#v, want [New]", spec.label.lines)
	}
	if !spec.labelIsExplicit {
		t.Fatal("expected A label to remain explicit")
	}
}

func TestMermaidFileToMapAssignsOwnerFromLexicalSubgraphAfterExternalReference(t *testing.T) {
	properties, err := mermaidFileToMap("graph LR\nGuest --> FuseServer\nsubgraph arcbox-fs\n    FuseServer --> Worker\nend", "cli")
	if err != nil {
		t.Fatalf("mermaidFileToMap() error = %v", err)
	}

	if len(properties.subgraphs) != 1 {
		t.Fatalf("subgraphs = %d, want 1", len(properties.subgraphs))
	}

	sg := properties.subgraphs[0]
	if properties.nodeOwners["FuseServer"] != sg {
		t.Fatal("expected FuseServer to belong to arcbox-fs")
	}
	if properties.nodeOwners["Worker"] != sg {
		t.Fatal("expected Worker to belong to arcbox-fs")
	}
	if len(sg.directNodes) != 2 || sg.directNodes[0] != "FuseServer" || sg.directNodes[1] != "Worker" {
		t.Fatalf("directNodes = %#v, want [FuseServer Worker]", sg.directNodes)
	}

	for _, item := range properties.rootItems {
		if item.kind == textGraphItemNode && item.nodeName == "FuseServer" {
			t.Fatal("did not expect FuseServer to remain a top-level item")
		}
	}
}

func TestMermaidFileToMapParsesSubgraphDirectionMetadata(t *testing.T) {
	properties, err := mermaidFileToMap("graph LR\nsubgraph Group\n    direction TB\n    A --> B\nend", "cli")
	if err != nil {
		t.Fatalf("mermaidFileToMap() error = %v", err)
	}

	if len(properties.subgraphs) != 1 {
		t.Fatalf("subgraphs = %d, want 1", len(properties.subgraphs))
	}

	if properties.subgraphs[0].direction != "TD" {
		t.Fatalf("direction = %q, want %q", properties.subgraphs[0].direction, "TD")
	}
	if _, ok := properties.nodeSpecs["direction TB"]; ok {
		t.Fatal("did not expect direction TB to be parsed as a node")
	}
}

func TestMermaidFileToMapKeepsSiblingOwnershipOnCrossSubgraphReference(t *testing.T) {
	properties, err := mermaidFileToMap("graph LR\nsubgraph Frontend\n    A\nend\nsubgraph Backend\n    B\n    A --> B\nend", "cli")
	if err != nil {
		t.Fatalf("mermaidFileToMap() error = %v", err)
	}

	if len(properties.subgraphs) != 2 {
		t.Fatalf("subgraphs = %d, want 2", len(properties.subgraphs))
	}

	frontend := properties.subgraphs[0]
	backend := properties.subgraphs[1]
	if properties.nodeOwners["A"] != frontend {
		t.Fatal("expected A to remain owned by Frontend")
	}
	if properties.nodeOwners["B"] != backend {
		t.Fatal("expected B to remain owned by Backend")
	}
}
