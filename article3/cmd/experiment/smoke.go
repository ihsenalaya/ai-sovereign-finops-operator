package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

type smokeCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type smokeOutput struct {
	ExperimentID string       `json:"experiment_id"`
	Variant      string       `json:"variant"`
	Checks       []smokeCheck `json:"checks"`
}

func runE0() error {
	output := smokeOutput{
		ExperimentID: "E0",
		Variant:      "local_scaffold_smoke",
		Checks: []smokeCheck{
			runCommandCheck("go_test", "go", "test", "./..."),
			pathCheck("status_md", "STATUS.md"),
			pathCheck("operator_audit", filepath.Join("operator_audit", "architecture.md")),
			pathCheck("experiment_registry", filepath.Join("provenance", "experiment_registry.csv")),
		},
	}

	if err := writeJSON(filepath.Join("experiments", "raw", "E0_smoke.json"), output); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join("experiments", "processed", "E0_smoke_summary.json"), output); err != nil {
		return err
	}
	return writeMarkdownReport(
		filepath.Join("reports", "E0_SMOKE_RESULTS.md"),
		"E0 Smoke Results",
		[]string{
			"Local smoke validation of the current GOV-AR scaffold.",
			"```json\n" + prettyJSON(output) + "\n```",
		},
	)
}

func runCommandCheck(name string, command string, args ...string) smokeCheck {
	cmd := exec.Command(command, args...)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		return smokeCheck{
			Name:   name,
			Status: "failed",
			Detail: string(out),
		}
	}
	return smokeCheck{
		Name:   name,
		Status: "passed",
		Detail: string(out),
	}
}

func pathCheck(name string, path string) smokeCheck {
	if _, err := os.Stat(path); err != nil {
		return smokeCheck{
			Name:   name,
			Status: "failed",
			Detail: err.Error(),
		}
	}
	return smokeCheck{
		Name:   name,
		Status: "passed",
		Detail: path,
	}
}
