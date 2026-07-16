package govarexperiment

import (
	"encoding/json"
	"strings"
	"testing"
)

func p1bDesignMatrixFixtureV2(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	encode := func(value any) json.RawMessage {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	return map[string]json.RawMessage{
		"seeds":              encode([]uint64{91001, 91002, 91003, 91004, 91005}),
		"scenarios":          encode([]string{"S0_reference", "S1_liability", "S2_tail", "S3_isolation"}),
		"selection_contract": encode(expectedP1bSelectionContractV2()),
	}
}

func TestP1bDesignMatrixAndSelectionContractRejectMutations(t *testing.T) {
	if err := validateP1bDesignMatrixAndSelectionV2(p1bDesignMatrixFixtureV2(t)); err != nil {
		t.Fatalf("exact P1b matrix/selection contract was rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]json.RawMessage)
		want   string
	}{
		{
			name: "seed-matrix",
			mutate: func(value map[string]json.RawMessage) {
				value["seeds"] = json.RawMessage(`[91001,91002,91003,91004,99999]`)
			},
			want: "five-seed matrix",
		},
		{
			name: "scenario-matrix",
			mutate: func(value map[string]json.RawMessage) {
				value["scenarios"] = json.RawMessage(`["S0_reference","S1_liability","S3_isolation","S2_tail"]`)
			},
			want: "four-scenario matrix",
		},
		{
			name: "selection-source-unit-domain",
			mutate: func(value map[string]json.RawMessage) {
				contract := expectedP1bSelectionContractV2()
				contract.SourceAssignmentIDDomain = "outcome-bound-domain"
				raw, _ := json.Marshal(contract)
				value["selection_contract"] = raw
			},
			want: "source-assignment v2",
		},
		{
			name: "selection-strict-json-type",
			mutate: func(value map[string]json.RawMessage) {
				var contract map[string]any
				_ = json.Unmarshal(value["selection_contract"], &contract)
				contract["paired_across_scenarios"] = "true"
				raw, _ := json.Marshal(contract)
				value["selection_contract"] = raw
			},
			want: "selection contract",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := p1bDesignMatrixFixtureV2(t)
			test.mutate(value)
			err := validateP1bDesignMatrixAndSelectionV2(value)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("mutation was not rejected precisely: %v", err)
			}
		})
	}
}
