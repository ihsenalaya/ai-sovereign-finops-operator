// Package workload loads the synthetic prompt datasets (W1-W4) used by the
// experiments. Each workload carries the metadata needed for routing and
// attribution (team, namespace, sensitivity, budget, allowed models).
package workload

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Prompt is one request in a workload dataset.
type Prompt struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Sensitive bool   `json:"sensitive"`
	System    string `json:"system,omitempty"`
}

// Workload is a family of realistic requests owned by a team/namespace.
type Workload struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Team             string   `json:"team"`
	Namespace        string   `json:"namespace"`
	DefaultSensitive bool     `json:"defaultSensitive"`
	MonthlyBudgetEUR float64  `json:"monthlyBudgetEUR"`
	PremiumModel     string   `json:"premiumModel"`
	AllowedModels    []string `json:"allowedModels"`
	MinQuality       float64  `json:"minQuality"`
	Prompts          []Prompt `json:"prompts"`
}

// Load reads every *.json workload file in dir, sorted by name for determinism.
func Load(dir string) ([]Workload, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	var out []Workload
	for _, p := range matches {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var w Workload
		if err := json.Unmarshal(b, &w); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

// TotalPrompts counts prompts across workloads.
func TotalPrompts(ws []Workload) int {
	n := 0
	for _, w := range ws {
		n += len(w.Prompts)
	}
	return n
}
