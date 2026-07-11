package main

import (
	"path/filepath"

	faultinjector "github.com/imperium/ai-sovereign-finops-operator/article3/src/fault_injector"
)

type faultExperimentOutput struct {
	ExperimentID       string                                      `json:"experiment_id"`
	Variant            string                                      `json:"variant"`
	DuplicateSettlement faultinjector.DuplicateSettlementResult    `json:"duplicate_settlement"`
	ReservationExpiry   faultinjector.ReservationExpiryResult      `json:"reservation_expiry"`
	TelemetryFault      faultinjector.TelemetryFaultResult         `json:"telemetry_fault"`
}

func runE4() error {
	output := faultExperimentOutput{
		ExperimentID:        "E4",
		Variant:             "fault_injection_scaffold",
		DuplicateSettlement: faultinjector.SimulateDuplicateSettlement(),
		ReservationExpiry:   faultinjector.SimulateReservationExpiry(),
		TelemetryFault:      faultinjector.SimulateTelemetryFault(),
	}
	if err := writeJSON(filepath.Join("experiments", "raw", "E4_faults.json"), output); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join("experiments", "processed", "E4_faults_summary.json"), output); err != nil {
		return err
	}
	return writeMarkdownReport(
		filepath.Join("reports", "E4_FAULT_RESULTS.md"),
		"E4 Fault Injection Results",
		[]string{
			"Deterministic fault injection over duplicate settlement, reservation expiry, and telemetry insufficiency.",
			"```json\n" + prettyJSON(output) + "\n```",
		},
	)
}
