package v1alpha1

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestAIWorkloadBindingSpecValidate(t *testing.T) {
	valid := AIWorkloadBindingSpec{
		ServiceAccountName: "payments-api",
		TenantID:           "tenant-a",
		Team:               "payments",
		Application:        "payments-api",
		BudgetPolicyRef:    "payments-budget",
		RoutingPolicyRef:   "payments-routing",
		Sensitivity:        TierHigh,
		AllowedZones:       []string{"eu", "francecentral"},
		RequireGateway:     true,
	}
	if errs := valid.Validate("payments-api", field.NewPath("spec")); len(errs) != 0 {
		t.Fatalf("valid binding returned errors: %v", errs)
	}

	invalid := valid
	invalid.TenantID = " "
	invalid.AllowedZones = []string{"EU", "eu", " francecentral"}
	errs := invalid.Validate("other-service-account", field.NewPath("spec"))
	joined := errs.ToAggregate().Error()
	for _, want := range []string{"tenantID", "must equal metadata.name", "lower-case", "Duplicate value"} {
		if !strings.Contains(joined, want) {
			t.Errorf("errors %q do not contain %q", joined, want)
		}
	}
}

func TestAdmissionApprovalDigestAndConsumptionIdentityBindImmutableInput(t *testing.T) {
	request := AIAdmissionApprovalRequest{RequestID: "request-1", Namespace: "finance", TenantID: "tenant-a", WorkloadUID: "workload-a",
		BudgetPolicy:      AIWorkloadBindingResolvedReference{Name: "budget", UID: "budget-uid", Generation: 2},
		RoutingPolicy:     AIWorkloadBindingResolvedReference{Name: "routing", UID: "routing-uid", Generation: 3},
		CandidateModelRef: "model", CandidateSnapshotVersion: strings.Repeat("b", 64), RouteSnapshot: GOVARRouteSnapshot{Namespace: "finance", ModelName: "model", ModelUID: "model-uid", ModelGeneration: 1, ModelResourceVersion: "m1", ProviderName: "provider", ProviderUID: "provider-uid", ProviderGeneration: 1, ProviderResourceVersion: "p1", PricingVersion: "prices-v1", PricingComplianceHash: strings.Repeat("a", 64), RouteBindingName: "primary", ProviderDeployment: "provider-model", Cluster: "backend", Authority: "backend.example", PathMode: "openai-body", SnapshotHash: strings.Repeat("b", 64)},
		InputTokens: 10, MaxOutputTokens: 100, ExpiresAt: metav1.NewTime(time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC))}
	request.RequestDigest = request.ComputeDigest()
	if len(request.RequestDigest) != 64 || request.RequestDigest != request.ComputeDigest() {
		t.Fatalf("unstable digest %q", request.RequestDigest)
	}
	altered := request
	altered.TenantID = "tenant-b"
	if altered.ComputeDigest() == request.RequestDigest {
		t.Fatal("tenant mutation did not change approval digest")
	}
	for index := 0; index < reflect.TypeOf(request.RouteSnapshot).NumField(); index++ {
		fieldName := reflect.TypeOf(request.RouteSnapshot).Field(index).Name
		changed := request
		field := reflect.ValueOf(&changed.RouteSnapshot).Elem().Field(index)
		switch field.Kind() {
		case reflect.String:
			field.SetString(field.String() + "x")
		case reflect.Int64:
			field.SetInt(field.Int() + 1)
		default:
			t.Fatalf("unhandled route snapshot field %s kind %s", fieldName, field.Kind())
		}
		if changed.ComputeDigest() == request.RequestDigest {
			t.Fatalf("route snapshot field %s did not change the one-request approval digest", fieldName)
		}
	}
	if AdmissionApprovalConsumptionName(types.UID("proposal-a")) == AdmissionApprovalConsumptionName(types.UID("proposal-b")) {
		t.Fatal("different proposal UIDs share a consumption Lease name")
	}
}

func TestAdmissionApprovalCRDsSeparateProposalAndHumanDecision(t *testing.T) {
	proposal, err := os.ReadFile("../../config/crd/bases/aiops.imperium.io_aiadmissionapprovals.yaml")
	if err != nil {
		t.Fatal(err)
	}
	decision, err := os.ReadFile("../../config/crd/bases/aiops.imperium.io_aiadmissionapprovaldecisions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"proposal": string(proposal), "decision": string(decision)} {
		if !strings.Contains(text, "spec is immutable") {
			t.Fatalf("%s CRD lacks immutable-spec CEL", name)
		}
	}
	if strings.Contains(string(proposal), "decision:\n") || !strings.Contains(string(decision), "proposalUID:") || !strings.Contains(string(decision), "requestDigest:") {
		t.Fatal("proposal and decision authorities are not structurally separated")
	}
}

func TestGOVARRoutingPolicySpecValidate(t *testing.T) {
	adaptive := int64(1200)
	valid := GOVARRoutingPolicySpec{
		Reservation: GOVARReservationPolicy{
			Method:                       GOVARReservationFixedCohort,
			AdaptiveQuantileOutputTokens: &adaptive,
		},
		Calibration: &GOVARCalibrationPolicy{
			ArtifactRef:    "sha256:calibration",
			Version:        "v1",
			MinimumSupport: 100,
			MaxAgeSeconds:  3600,
		},
		Drift: GOVARDriftPolicy{
			Detector:                   "coverage-gap",
			ThresholdPPB:               10_000_000,
			Fallback:                   "strict_provider_cap",
			RevalidationMinimumSupport: 100,
		},
		Cohort: &GOVARCohortPolicy{
			RegistryRef:        "cohort-a",
			Size:               10,
			OpportunitySetHash: strings.Repeat("a", 64),
			WeightsHash:        strings.Repeat("b", 64),
		},
		Risk: &GOVARRiskPolicy{TenantRiskPPB: 1_000_000, Allocation: "uniform"},
	}
	if errs := valid.Validate(field.NewPath("spec", "govar")); len(errs) != 0 {
		t.Fatalf("valid GOV-AR policy returned errors: %v", errs)
	}

	invalid := valid
	invalid.Reservation.AdaptiveQuantileOutputTokens = nil
	invalid.Calibration = nil
	invalid.Cohort = nil
	invalid.Risk = nil
	errs := invalid.Validate(field.NewPath("spec", "govar"))
	if len(errs) != 4 {
		t.Fatalf("invalid GOV-AR policy errors = %d, want 4: %v", len(errs), errs)
	}
}
