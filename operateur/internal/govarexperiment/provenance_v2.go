package govarexperiment

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	E1PreOutcomeExecutionSubjectDomainV2 = "govar-e1-p1b-preoutcome-execution-subject-v1"
	E1ExecutionLockDomainV2              = "govar-e1-p1b-execution-lock-v2"
	AzurePreOutcomeIDDomainV2            = "article3-azure-llm-code-preoutcome-id-v1"
	PreOutcomeMappingSchemaV2            = "govar-e1-preoutcome-mapping-v2"
	streamPreOutcomeDigestSchemaV2       = "sha256-newline-delimited-preoutcome-id-in-stream-sequence-v1"
)

type E1SupportModeV2 string

const (
	E1SupportModeFixture  E1SupportModeV2 = "fixture"
	E1SupportModeP1bExact E1SupportModeV2 = "p1b_exact"
)

var azureSourceTimestampPatternV2 = regexp.MustCompile(
	`^[0-9]{4}-[0-9]{2}-[0-9]{2}[ T][0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-][0-9]{2}:[0-9]{2})$`,
)

// E1ProvenanceLockV2 is comparable by construction. The same exact value is
// carried by ConfigV2, UsageProducerEvidenceV2, and UsageBindingV2. The final
// usage-binding digest and revealed usage are deliberately outside this
// structure. ExecutionSubjectSHA256 contains only pre-outcome/pre-read design
// inputs. The external authorization binds that subject; only its exact byte
// hash is then combined with the subject to form ExecutionLockSHA256. This
// two-stage construction prevents the authorization/lock hash cycle.
type E1ProvenanceLockV2 struct {
	SupportMode                    E1SupportModeV2 `json:"support_mode"`
	OpportunityStreamSHA256        string          `json:"opportunity_stream_sha256"`
	SourceCodeBindingSHA256        string          `json:"source_code_binding_sha256"`
	SourceManifestSHA256           string          `json:"source_manifest_sha256"`
	PilotUsageBindingSHA256        string          `json:"pilot_usage_binding_sha256"`
	RuntimeCapabilitySHA256        string          `json:"runtime_capability_sha256"`
	MappingArtifactSHA256          string          `json:"mapping_artifact_sha256"`
	PilotPreOutcomeSetSHA256       string          `json:"pilot_preoutcome_set_sha256"`
	AssignedPreOutcomeSetSHA256    string          `json:"assigned_preoutcome_set_sha256"`
	StreamPreOutcomeSequenceSHA256 string          `json:"stream_preoutcome_sequence_sha256"`
	ProtocolSHA256                 string          `json:"protocol_sha256"`
	DesignSpecSHA256               string          `json:"design_spec_sha256"`
	DecisionConfigTemplateSHA256   string          `json:"decision_config_template_sha256"`
	DesignLockSHA256               string          `json:"design_lock_sha256"`
	IndependentDesignReviewSHA256  string          `json:"independent_design_review_sha256"`
	DesignManifestSHA256           string          `json:"design_manifest_sha256"`
	CohortRegistrySHA256           string          `json:"cohort_registry_sha256"`
	ProfileExecutionContractSHA256 string          `json:"profile_execution_contract_sha256"`
	BudgetCalibrationSHA256        string          `json:"budget_calibration_sha256"`
	CalibrationArtifactSHA256      string          `json:"calibration_artifact_sha256"`
	CandidateSetSHA256             string          `json:"candidate_set_sha256"`
	ProfileRegistrySHA256          string          `json:"profile_registry_sha256"`
	SlotTemplateSHA256             string          `json:"slot_template_sha256"`
	MethodBuilderBundleSHA256      string          `json:"method_builder_bundle_sha256"`
	ExecutionIntentSHA256          string          `json:"execution_intent_sha256"`
	ExecutionSubjectSHA256         string          `json:"execution_subject_sha256"`
	IndependentAuthorizationSHA256 string          `json:"independent_authorization_sha256"`
	ExecutionLockSHA256            string          `json:"execution_lock_sha256"`
}

func (l E1ProvenanceLockV2) preOutcomeExecutionSubjectDigest() string {
	return DomainHash(E1PreOutcomeExecutionSubjectDomainV2,
		[]byte(l.SupportMode),
		[]byte(l.OpportunityStreamSHA256),
		[]byte(l.SourceCodeBindingSHA256),
		[]byte(l.SourceManifestSHA256),
		[]byte(l.PilotUsageBindingSHA256),
		[]byte(l.RuntimeCapabilitySHA256),
		[]byte(l.MappingArtifactSHA256),
		[]byte(l.PilotPreOutcomeSetSHA256),
		[]byte(l.AssignedPreOutcomeSetSHA256),
		[]byte(l.StreamPreOutcomeSequenceSHA256),
		[]byte(l.ProtocolSHA256),
		[]byte(l.DesignSpecSHA256),
		[]byte(l.DecisionConfigTemplateSHA256),
		[]byte(l.DesignLockSHA256),
		[]byte(l.IndependentDesignReviewSHA256),
		[]byte(l.DesignManifestSHA256),
		[]byte(l.CohortRegistrySHA256),
		[]byte(l.ProfileExecutionContractSHA256),
		[]byte(l.BudgetCalibrationSHA256),
		[]byte(l.CalibrationArtifactSHA256),
		[]byte(l.CandidateSetSHA256),
		[]byte(l.ProfileRegistrySHA256),
		[]byte(l.SlotTemplateSHA256),
		[]byte(l.MethodBuilderBundleSHA256),
		[]byte(l.ExecutionIntentSHA256),
	)
}

func (l E1ProvenanceLockV2) executionDigest() string {
	return DomainHash(E1ExecutionLockDomainV2,
		[]byte(l.ExecutionSubjectSHA256),
		[]byte(l.IndependentAuthorizationSHA256),
	)
}

func (l E1ProvenanceLockV2) Validate() error {
	switch l.SupportMode {
	case E1SupportModeFixture, E1SupportModeP1bExact:
	default:
		return fmt.Errorf("unsupported E1 support_mode %q", l.SupportMode)
	}
	for name, value := range map[string]string{
		"opportunity_stream_sha256":         l.OpportunityStreamSHA256,
		"source_code_binding_sha256":        l.SourceCodeBindingSHA256,
		"source_manifest_sha256":            l.SourceManifestSHA256,
		"pilot_usage_binding_sha256":        l.PilotUsageBindingSHA256,
		"runtime_capability_sha256":         l.RuntimeCapabilitySHA256,
		"mapping_artifact_sha256":           l.MappingArtifactSHA256,
		"pilot_preoutcome_set_sha256":       l.PilotPreOutcomeSetSHA256,
		"assigned_preoutcome_set_sha256":    l.AssignedPreOutcomeSetSHA256,
		"stream_preoutcome_sequence_sha256": l.StreamPreOutcomeSequenceSHA256,
		"protocol_sha256":                   l.ProtocolSHA256,
		"design_spec_sha256":                l.DesignSpecSHA256,
		"decision_config_template_sha256":   l.DecisionConfigTemplateSHA256,
		"design_lock_sha256":                l.DesignLockSHA256,
		"independent_design_review_sha256":  l.IndependentDesignReviewSHA256,
		"design_manifest_sha256":            l.DesignManifestSHA256,
		"cohort_registry_sha256":            l.CohortRegistrySHA256,
		"profile_execution_contract_sha256": l.ProfileExecutionContractSHA256,
		"budget_calibration_sha256":         l.BudgetCalibrationSHA256,
		"calibration_artifact_sha256":       l.CalibrationArtifactSHA256,
		"candidate_set_sha256":              l.CandidateSetSHA256,
		"profile_registry_sha256":           l.ProfileRegistrySHA256,
		"slot_template_sha256":              l.SlotTemplateSHA256,
		"method_builder_bundle_sha256":      l.MethodBuilderBundleSHA256,
		"execution_intent_sha256":           l.ExecutionIntentSHA256,
		"execution_subject_sha256":          l.ExecutionSubjectSHA256,
		"independent_authorization_sha256":  l.IndependentAuthorizationSHA256,
		"execution_lock_sha256":             l.ExecutionLockSHA256,
	} {
		if !shaPattern.MatchString(value) {
			return fmt.Errorf("%s must be a lower-case SHA-256", name)
		}
	}
	if l.ExecutionSubjectSHA256 != l.preOutcomeExecutionSubjectDigest() {
		return errors.New("execution_subject_sha256 does not recompute from the pre-outcome support set")
	}
	if l.ExecutionLockSHA256 != l.executionDigest() {
		return errors.New("execution_lock_sha256 does not recompute from subject plus independent authorization")
	}
	return nil
}

// AzurePreOutcomeIDV2 exactly reproduces azure_preoutcome_id in
// article3/datasets/subsplit_azure_development.py. SourceTimestamp is hashed as
// supplied; it is validated but never parsed, normalized, or reformatted.
func AzurePreOutcomeIDV2(sourceRow int64, sourceTimestamp string, contextTokens int64) (string, error) {
	if sourceRow < 0 || contextTokens < 0 {
		return "", errors.New("Azure pre-outcome coordinates must be non-negative")
	}
	if !azureSourceTimestampPatternV2.MatchString(sourceTimestamp) {
		return "", errors.New("Azure source_timestamp is invalid")
	}
	// Parse only to validate the real calendar/clock/offset. The producer's
	// original timestamp bytes (including its space-or-T separator and exact
	// fractional representation) remain untouched in the hash payload below.
	if _, err := time.Parse(time.RFC3339Nano, strings.Replace(sourceTimestamp, " ", "T", 1)); err != nil {
		return "", errors.New("Azure source_timestamp is invalid")
	}
	payload := AzurePreOutcomeIDDomainV2 + "\x00" + UsageDatasetV2 + "\x00" +
		strconv.FormatInt(sourceRow, 10) + "\x00" + sourceTimestamp + "\x00" +
		strconv.FormatInt(contextTokens, 10)
	return SHA256([]byte(payload)), nil
}

func validatePreOutcomeCoordinatesV2(sourceRow int64, sourceTimestamp string, contextTokens int64, preOutcomeID, usageItemID string) error {
	expected, err := AzurePreOutcomeIDV2(sourceRow, sourceTimestamp, contextTokens)
	if err != nil {
		return err
	}
	if !shaPattern.MatchString(preOutcomeID) || preOutcomeID != expected {
		return errors.New("preoutcome_id does not recompute from the exact Azure pre-outcome coordinates")
	}
	if usageItemID != preOutcomeID {
		return errors.New("usage_item_id must equal preoutcome_id")
	}
	return nil
}

// rejectDuplicateTopLevelJSONKeysV2 closes encoding/json's last-key-wins gap
// for the flat E1-v2 stream, mapping, and usage row contracts.
func rejectDuplicateTopLevelJSONKeysV2(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return errors.New("E1 v2 row must be one JSON object")
	}
	seen := map[string]struct{}{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return errors.New("E1 v2 object key is not a string")
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("E1 v2 JSON object duplicates key %q", key)
		}
		seen[key] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

// StreamPreOutcomeSequenceSHA256V2 follows the dataset producer's ordered-ID
// convention: SHA-256 over each lower-case ID followed by a single LF, in
// contiguous OpportunityV2.Sequence order. No sorting is permitted here.
func StreamPreOutcomeSequenceSHA256V2(opportunities []OpportunityV2) (string, error) {
	var payload bytes.Buffer
	for index, opportunity := range opportunities {
		if opportunity.Sequence != int64(index) {
			return "", fmt.Errorf("pre-outcome sequence row %d is not contiguous", index+1)
		}
		if !shaPattern.MatchString(opportunity.PreOutcomeID) {
			return "", fmt.Errorf("pre-outcome sequence row %d has an invalid identifier", index+1)
		}
		payload.WriteString(opportunity.PreOutcomeID)
		payload.WriteByte('\n')
	}
	if len(opportunities) == 0 {
		return "", errors.New("pre-outcome sequence is empty")
	}
	return SHA256(payload.Bytes()), nil
}

// StreamPreOutcomeSetSHA256V2 follows the pretrace producer's set convention:
// SHA-256 over the unique lower-case pre-outcome IDs in lexical order, with
// one LF after every identifier. This is intentionally distinct from the
// opportunity sequence digest above.
func StreamPreOutcomeSetSHA256V2(opportunities []OpportunityV2) (string, error) {
	identifiers := make([]string, 0, len(opportunities))
	seen := make(map[string]struct{}, len(opportunities))
	for index, opportunity := range opportunities {
		if !shaPattern.MatchString(opportunity.PreOutcomeID) {
			return "", fmt.Errorf("pre-outcome set row %d has an invalid identifier", index+1)
		}
		if _, duplicate := seen[opportunity.PreOutcomeID]; duplicate {
			return "", fmt.Errorf("pre-outcome set row %d duplicates identifier %s", index+1, opportunity.PreOutcomeID)
		}
		seen[opportunity.PreOutcomeID] = struct{}{}
		identifiers = append(identifiers, opportunity.PreOutcomeID)
	}
	if len(identifiers) == 0 {
		return "", errors.New("pre-outcome set is empty")
	}
	sort.Strings(identifiers)
	var payload bytes.Buffer
	for _, identifier := range identifiers {
		payload.WriteString(identifier)
		payload.WriteByte('\n')
	}
	return SHA256(payload.Bytes()), nil
}

// PreOutcomeMappingV2 has an outcome-field-free wire schema for the exact
// mapping from Azure source coordinates to E1 opportunity fields declared as
// fixed before admission. Runtime validation proves structural absence of
// event_id, generated_tokens, and actual usage, plus exact byte/lock binding;
// it cannot prove that a producer did not choose otherwise valid field values
// after observing outcomes. That temporal claim remains external, independently
// authorized provenance, and this qualification runtime is non-final evidence.
type PreOutcomeMappingV2 struct {
	SchemaVersion               string `json:"schema_version"`
	RecordType                  string `json:"record_type"`
	Sequence                    int64  `json:"sequence"`
	RequestID                   string `json:"request_id"`
	TenantID                    string `json:"tenant_id"`
	BudgetWindowID              string `json:"budget_window_id"`
	WorkloadUID                 string `json:"workload_uid"`
	BudgetMicros                int64  `json:"budget_micros"`
	CohortID                    string `json:"cohort_id,omitempty"`
	CohortIndex                 int64  `json:"cohort_index,omitempty"`
	InputTokens                 int64  `json:"input_tokens"`
	MaxOutputTokens             int64  `json:"max_output_tokens"`
	ArrivalAt                   string `json:"arrival_at"`
	ProviderResponseAt          string `json:"provider_response_at"`
	UsageAvailableAt            string `json:"usage_available_at"`
	SettlementAt                string `json:"settlement_at"`
	FaultMode                   string `json:"fault_mode"`
	UsageItemID                 string `json:"usage_item_id"`
	SourceRow                   int64  `json:"source_row"`
	SourceTimestamp             string `json:"source_timestamp"`
	ContextTokens               int64  `json:"context_tokens"`
	PreOutcomeID                string `json:"preoutcome_id"`
	DuplicateSettlement         bool   `json:"duplicate_settlement,omitempty"`
	ConflictingSettlementReplay bool   `json:"conflicting_settlement_replay,omitempty"`
}

func (m PreOutcomeMappingV2) Opportunity() OpportunityV2 {
	return OpportunityV2{
		SchemaVersion: OpportunitySchemaV2, RecordType: "opportunity", Sequence: m.Sequence,
		RequestID: m.RequestID, TenantID: m.TenantID, BudgetWindowID: m.BudgetWindowID,
		WorkloadUID: m.WorkloadUID, BudgetMicros: m.BudgetMicros, CohortID: m.CohortID,
		CohortIndex: m.CohortIndex, InputTokens: m.InputTokens, MaxOutputTokens: m.MaxOutputTokens,
		ArrivalAt: m.ArrivalAt, ProviderResponseAt: m.ProviderResponseAt,
		UsageAvailableAt: m.UsageAvailableAt, SettlementAt: m.SettlementAt,
		FaultMode: m.FaultMode, UsageItemID: m.UsageItemID,
		SourceRow: m.SourceRow, SourceTimestamp: m.SourceTimestamp,
		ContextTokens: m.ContextTokens, PreOutcomeID: m.PreOutcomeID,
		DuplicateSettlement:         m.DuplicateSettlement,
		ConflictingSettlementReplay: m.ConflictingSettlementReplay,
	}
}

func (m PreOutcomeMappingV2) Validate() error {
	if m.SchemaVersion != PreOutcomeMappingSchemaV2 || m.RecordType != "preoutcome_mapping" {
		return errors.New("unsupported E1 v2 pre-outcome mapping schema or record type")
	}
	if err := m.Opportunity().Validate(); err != nil {
		return err
	}
	return nil
}

func ParsePreOutcomeMappingV2(raw []byte) ([]PreOutcomeMappingV2, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	rows := make([]PreOutcomeMappingV2, 0)
	requests := map[string]struct{}{}
	items := map[string]struct{}{}
	sourceRows := map[int64]struct{}{}
	for scanner.Scan() {
		lineNumber := len(rows) + 1
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			return nil, fmt.Errorf("pre-outcome mapping line %d is empty", lineNumber)
		}
		if err := rejectDuplicateTopLevelJSONKeysV2(line); err != nil {
			return nil, fmt.Errorf("pre-outcome mapping line %d: %w", lineNumber, err)
		}
		var row PreOutcomeMappingV2
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&row); err != nil {
			return nil, fmt.Errorf("pre-outcome mapping line %d: %w", lineNumber, err)
		}
		if err := requireJSONEOF(decoder); err != nil {
			return nil, fmt.Errorf("pre-outcome mapping line %d: %w", lineNumber, err)
		}
		if err := row.Validate(); err != nil {
			return nil, fmt.Errorf("pre-outcome mapping line %d: %w", lineNumber, err)
		}
		if row.Sequence != int64(len(rows)) {
			return nil, fmt.Errorf("pre-outcome mapping line %d must have contiguous sequence %d", lineNumber, len(rows))
		}
		if _, duplicate := requests[row.RequestID]; duplicate {
			return nil, fmt.Errorf("pre-outcome mapping line %d duplicates request_id %q", lineNumber, row.RequestID)
		}
		if _, duplicate := items[row.PreOutcomeID]; duplicate {
			return nil, fmt.Errorf("pre-outcome mapping line %d duplicates preoutcome_id %q", lineNumber, row.PreOutcomeID)
		}
		if _, duplicate := sourceRows[row.SourceRow]; duplicate {
			return nil, fmt.Errorf("pre-outcome mapping line %d duplicates source_row %d", lineNumber, row.SourceRow)
		}
		requests[row.RequestID] = struct{}{}
		items[row.PreOutcomeID] = struct{}{}
		sourceRows[row.SourceRow] = struct{}{}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read pre-outcome mapping: %w", err)
	}
	if len(rows) == 0 {
		return nil, errors.New("pre-outcome mapping artifact is empty")
	}
	return rows, nil
}

func canonicalPreOutcomeMappingSetSHA256V2(rows []PreOutcomeMappingV2) string {
	ordered := append([]PreOutcomeMappingV2(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].RequestID < ordered[j].RequestID })
	raw, _ := json.Marshal(struct {
		Schema string                `json:"schema"`
		Rows   []PreOutcomeMappingV2 `json:"rows"`
	}{"govar-e1-preoutcome-mapping-set-v2", ordered})
	return DomainHash("govar-e1-preoutcome-mapping-set-v2", raw)
}
