package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
	"github.com/imperium/ai-sovereign-finops-operator/internal/webhook/podinjector"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type tokenReviewClient struct {
	client.Client
	status authenticationv1.TokenReviewStatus
	spec   authenticationv1.TokenReviewSpec
}

func (c *tokenReviewClient) Create(ctx context.Context, object client.Object, opts ...client.CreateOption) error {
	if review, ok := object.(*authenticationv1.TokenReview); ok {
		c.spec = review.Spec
		review.Status = c.status
		return nil
	}
	return c.Client.Create(ctx, object, opts...)
}

func validTokenReviewStatus() authenticationv1.TokenReviewStatus {
	return authenticationv1.TokenReviewStatus{Authenticated: true, Audiences: []string{podinjector.GOVARTokenAudience}, User: authenticationv1.UserInfo{
		Username: "system:serviceaccount:finance:worker",
		Extra: map[string]authenticationv1.ExtraValue{
			"authentication.kubernetes.io/pod-name": {"agent"},
			"authentication.kubernetes.io/pod-uid":  {"uid-a"},
		},
	}}
}

func TestTokenReviewRequiresPodBindingAndDedicatedAudience(t *testing.T) {
	base := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
	tests := []struct {
		name   string
		mutate func(*authenticationv1.TokenReviewStatus)
		ok     bool
	}{
		{name: "valid", ok: true},
		{name: "unauthenticated", mutate: func(s *authenticationv1.TokenReviewStatus) { s.Authenticated = false }},
		{name: "wrong audience", mutate: func(s *authenticationv1.TokenReviewStatus) { s.Audiences = []string{"kubernetes.default.svc"} }},
		{name: "missing pod uid", mutate: func(s *authenticationv1.TokenReviewStatus) {
			delete(s.User.Extra, "authentication.kubernetes.io/pod-uid")
		}},
		{name: "missing pod name", mutate: func(s *authenticationv1.TokenReviewStatus) {
			delete(s.User.Extra, "authentication.kubernetes.io/pod-name")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := validTokenReviewStatus()
			if test.mutate != nil {
				test.mutate(&status)
			}
			reviewer := &tokenReviewClient{Client: base, status: status}
			principal, err := tokenReviewFunc(reviewer)(context.Background(), "bound-token")
			if (err == nil) != test.ok {
				t.Fatalf("principal=%+v err=%v", principal, err)
			}
			if len(reviewer.spec.Audiences) != 1 || reviewer.spec.Audiences[0] != podinjector.GOVARTokenAudience {
				t.Fatalf("TokenReview audience=%v", reviewer.spec.Audiences)
			}
		})
	}
}

type countingBackend struct {
	*govar.Engine
	dispatchCalls int
	settleCalls   int
}

func (b *countingBackend) Dispatch(req govar.DispatchRequest) (govar.Reservation, govar.ReasonCode, error) {
	b.dispatchCalls++
	return b.Engine.Dispatch(req)
}

func (b *countingBackend) Settle(req govar.SettleRequest) (govar.Reservation, govar.ReasonCode, error) {
	b.settleCalls++
	return b.Engine.Settle(req)
}

func TestWorkloadTokenCannotForgeZeroUsageFinalSettlement(t *testing.T) {
	backend := &countingBackend{Engine: govar.NewEngine()}
	srv := &server{engine: backend, auth: identityAuthenticator{reviewToken: func(context.Context, string) (authenticatedPrincipal, error) {
		return authenticatedPrincipal{namespace: "finance", workloadUID: "uid-a", podName: "agent", serviceAccount: "worker", role: roleAdmissionOnly}, nil
	}}}
	body := []byte(`{"request_id":"r1","settlement_id":"forged-final","tenant_id":"tenant-a","workload_uid":"uid-a","provider_attempt_id":"r1:attempt:1","actual_input_tokens":0,"actual_output_tokens":0,"usage_version":1,"final":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/settle", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer valid-bound-token")
	recorder := httptest.NewRecorder()
	srv.handleSettle(recorder, req)
	if recorder.Code != http.StatusForbidden || backend.settleCalls != 0 {
		t.Fatalf("status=%d settle_calls=%d body=%s", recorder.Code, backend.settleCalls, recorder.Body.String())
	}
	liability, err := backend.LiabilityWithError("tenant-a")
	if err != nil || liability.SettledSpendMicros != 0 || liability.OutstandingLiabilityMicros != 0 {
		t.Fatalf("ledger changed after rejected workload settlement: liability=%+v err=%v", liability, err)
	}
}

func TestAuthenticatedBodyOverOneMiBIsRejectedExplicitly(t *testing.T) {
	srv := &server{engine: govar.NewEngine(), auth: identityAuthenticator{reviewToken: func(context.Context, string) (authenticatedPrincipal, error) {
		return authenticatedPrincipal{role: roleAdmissionOnly}, nil
	}}}
	req := httptest.NewRequest(http.MethodPost, "/v1/admit", bytes.NewReader(bytes.Repeat([]byte("x"), int(maxAuthenticatedBodyBytes)+1)))
	req.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	srv.handleAdmit(recorder, req)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestBoundPodCannotMutateAnotherTenantWithForgedHeaderOrBody(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "finance", UID: types.UID("uid-a"),
		Annotations: map[string]string{podinjector.GOVARTenantKey: "tenant-a", podinjector.GOVARBudgetPolicyKey: "budget", podinjector.GOVARRoutingKey: "routing"}}}
	backend := &countingBackend{Engine: govar.NewEngine()}
	srv := &server{k8s: fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build(), engine: backend,
		auth: identityAuthenticator{reviewToken: func(context.Context, string) (authenticatedPrincipal, error) {
			return authenticatedPrincipal{namespace: "finance", workloadUID: "uid-a", podName: "agent", serviceAccount: "worker", role: roleAdmissionOnly}, nil
		}}}
	body := []byte(`{"request_id":"r1","event_id":"e1","tenant_id":"victim","workload_uid":"uid-a","provider_attempt_id":"a1","status":"CLAIMED"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/dispatch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer valid-bound-token")
	req.Header.Set("X-GOVAR-Tenant-ID", "victim")
	recorder := httptest.NewRecorder()
	srv.handleDispatch(recorder, req)
	if recorder.Code != http.StatusForbidden || backend.dispatchCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, backend.dispatchCalls, recorder.Body.String())
	}
}

func TestDeletedBoundPodIsRejected(t *testing.T) {
	srv := &server{k8s: fakeclient.NewClientBuilder().WithScheme(scheme).Build()}
	_, err := srv.resolveTrustedWorkload(context.Background(), authenticatedPrincipal{namespace: "finance", workloadUID: "uid-a", podName: "deleted"})
	if err == nil {
		t.Fatal("deleted bound Pod was accepted")
	}
}

func TestIdentityAuthenticatorAcceptsValidBoundBody(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	auth := identityAuthenticator{masterSecret: []byte("0123456789abcdef0123456789abcdef"), now: func() time.Time { return now }}
	body := []byte(`{"tenant_id":"tenant-a","workload_uid":"uid-a"}`)
	req := signedRequest(t, deriveSignerKey(auth.masterSecret, "finance", "tenant-a", "uid-a"), now, "/v1/admit", "tenant-a", "uid-a", body)
	principal, gotBody, err := auth.authenticate(req)
	if err != nil {
		t.Fatal(err)
	}
	if principal.tenantID != "tenant-a" || principal.workloadUID != "uid-a" || !bytes.Equal(gotBody, body) {
		t.Fatalf("principal=%+v body=%q", principal, gotBody)
	}
}

type failingReadyBackend struct{ *govar.Engine }

func (f failingReadyBackend) Ready(context.Context) error { return errors.New("database unavailable") }

func TestReadyzReportsLedgerFailure(t *testing.T) {
	srv := &server{engine: failingReadyBackend{Engine: govar.NewEngine()}}
	recorder := httptest.NewRecorder()
	srv.handleReadyz(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable || !bytes.Contains(recorder.Body.Bytes(), []byte("ledger not ready")) {
		t.Fatalf("readyz=%d %s", recorder.Code, recorder.Body.String())
	}
}

func TestIdentityAuthenticatorRejectsBodyTamperingAndReplay(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	auth := identityAuthenticator{masterSecret: []byte("0123456789abcdef0123456789abcdef"), now: func() time.Time { return now }}
	signer := deriveSignerKey(auth.masterSecret, "finance", "tenant-a", "uid-a")
	req := signedRequest(t, signer, now, "/v1/settle", "tenant-a", "uid-a", []byte(`{"request_id":"r1"}`))
	req.Body = http.NoBody
	if _, _, err := auth.authenticate(req); err == nil {
		t.Fatal("tampered body was accepted")
	}
	old := signedRequest(t, signer, now.Add(-3*time.Minute), "/v1/admit", "tenant-a", "uid-a", nil)
	if _, _, err := auth.authenticate(old); err == nil {
		t.Fatal("replayed signature was accepted")
	}
}

func TestDerivedSignerCannotImpersonateAnotherPrincipal(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	master := []byte("0123456789abcdef0123456789abcdef")
	auth := identityAuthenticator{masterSecret: master, now: func() time.Time { return now }}
	tenantASigner := deriveSignerKey(master, "finance", "tenant-a", "uid-a")
	req := signedRequest(t, tenantASigner, now, "/v1/admit", "tenant-b", "uid-b", nil)
	if _, _, err := auth.authenticate(req); err == nil {
		t.Fatal("per-principal signer impersonated another principal")
	}
}

func TestPublicGatewayCannotClaimAuthoritativeUnbilled(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	master := []byte("0123456789abcdef0123456789abcdef")
	srv := &server{engine: govar.NewEngine(), auth: identityAuthenticator{masterSecret: master, now: func() time.Time { return now }}}
	body := []byte(`{"request_id":"r1","event_id":"c1","tenant_id":"tenant-a","workload_uid":"uid-a","authoritative_unbilled":true}`)
	req := signedRequest(t, deriveSignerKey(master, "finance", "tenant-a", "uid-a"), now, "/v1/cancel", "tenant-a", "uid-a", body)
	recorder := httptest.NewRecorder()
	srv.handleCancel(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicGatewayCannotPostArbitrarySettlementCost(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	master := []byte("0123456789abcdef0123456789abcdef")
	srv := &server{engine: govar.NewEngine(), auth: identityAuthenticator{masterSecret: master, now: func() time.Time { return now }}}
	body := []byte(`{"request_id":"r1","settlement_id":"s1","tenant_id":"tenant-a","workload_uid":"uid-a","actual_cost_micros":99,"usage_version":1,"final":true}`)
	req := signedRequest(t, deriveSignerKey(master, "finance", "tenant-a", "uid-a"), now, "/v1/settle", "tenant-a", "uid-a", body)
	recorder := httptest.NewRecorder()
	srv.handleSettle(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestResolveTrustedWorkloadUsesActualPodUIDAndMetadata(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "finance", UID: types.UID("uid-a"), ResourceVersion: "7",
		Labels: map[string]string{"aiops.imperium.io/team": "treasury"}, Annotations: map[string]string{
			"aiops.imperium.io/govar-tenant": "tenant-a", "aiops.imperium.io/govar-budget-policy": "budget",
			"aiops.imperium.io/govar-routing-policy": "routing", "aiops.imperium.io/govar-sensitive-data": "true",
			"aiops.imperium.io/govar-allowed-zones": "eu,francecentral"}}, Spec: corev1.PodSpec{ServiceAccountName: "worker"}}
	ready := []metav1.Condition{{Type: aiopsv1alpha1.ConditionReady, Status: metav1.ConditionTrue}}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "finance", UID: types.UID("sa-uid")}}
	budget := &aiopsv1alpha1.AIBudgetPolicy{ObjectMeta: metav1.ObjectMeta{Name: "budget", Namespace: "finance", UID: types.UID("budget-uid"), Generation: 2}, Status: aiopsv1alpha1.AIBudgetPolicyStatus{ObservedGeneration: 2, Conditions: ready}}
	routing := &aiopsv1alpha1.AIRoutingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "routing", Namespace: "finance", UID: types.UID("routing-uid"), Generation: 3}, Status: aiopsv1alpha1.AIRoutingPolicyStatus{ObservedGeneration: 3, Conditions: ready}}
	binding := &aiopsv1alpha1.AIWorkloadBinding{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "finance", UID: types.UID("binding-uid"), Generation: 1, ResourceVersion: "8"},
		Spec:   aiopsv1alpha1.AIWorkloadBindingSpec{ServiceAccountName: "worker", TenantID: "tenant-a", Team: "treasury", Application: "assistant", BudgetPolicyRef: "budget", RoutingPolicyRef: "routing", Sensitivity: aiopsv1alpha1.TierHigh, AllowedZones: []string{"eu", "francecentral"}, RequireGateway: true},
		Status: aiopsv1alpha1.AIWorkloadBindingStatus{ObservedGeneration: 1, ResolvedServiceAccountUID: "sa-uid", ResolvedBudgetPolicy: &aiopsv1alpha1.AIWorkloadBindingResolvedReference{Name: "budget", UID: "budget-uid", Generation: 2}, ResolvedRoutingPolicy: &aiopsv1alpha1.AIWorkloadBindingResolvedReference{Name: "routing", UID: "routing-uid", Generation: 3}, Conditions: ready}}
	srv := &server{k8s: fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(pod, sa, budget, routing, binding).Build()}
	trusted, err := srv.resolveTrustedWorkload(context.Background(), authenticatedPrincipal{namespace: "finance", tenantID: "tenant-a", workloadUID: "uid-a", serviceAccount: "worker"})
	if err != nil || trusted.resourceVersion != "7|binding-uid|1|8" || trusted.team != "treasury" || !trusted.sensitive {
		t.Fatalf("trusted=%+v err=%v", trusted, err)
	}
	req := govar.AdmitRequest{Namespace: "finance", TenantID: "tenant-a", WorkloadUID: "uid-a", Team: "treasury", Application: "assistant", BudgetPolicyName: "budget", RoutingPolicyName: "routing", SensitiveData: true, AllowedZones: []string{"francecentral", "eu"}}
	if err := bindTrustedAdmitRequest(&req, trusted); err != nil {
		t.Fatal(err)
	}
	req.SensitiveData = false
	if err := bindTrustedAdmitRequest(&req, trusted); err == nil {
		t.Fatal("caller relaxed trusted sensitivity")
	}
	binding.Spec.RequireGateway = false
	if err := srv.k8s.Update(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.resolveTrustedWorkload(context.Background(), authenticatedPrincipal{namespace: "finance", workloadUID: "uid-a", serviceAccount: "worker"}); err == nil {
		t.Fatal("binding with requireGateway=false entered synchronous GOV-AR enforcement")
	}
	binding.Spec.RequireGateway = true
	if err := srv.k8s.Update(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	pod.Annotations[podinjector.GOVARTenantKey] = "victim"
	if err := srv.k8s.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.resolveTrustedWorkload(context.Background(), authenticatedPrincipal{namespace: "finance", workloadUID: "uid-a", serviceAccount: "worker"}); err == nil {
		t.Fatal("cross-tenant Pod annotation was accepted over AIWorkloadBinding")
	}
}

func TestDecodeStrictJSONRejectsUnknownAndMultipleValues(t *testing.T) {
	var dst struct {
		Known string `json:"known"`
	}
	if err := decodeStrictJSON([]byte(`{"known":"yes","unknown":1}`), &dst); err == nil {
		t.Fatal("unknown field accepted")
	}
	if err := decodeStrictJSON([]byte(`{"known":"yes"} {"known":"twice"}`), &dst); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
}

func TestAdmissionApprovalIsOneUseAndCrashSafe(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	req := govar.AdmitRequest{RequestID: "request-1", Namespace: "finance", TenantID: "tenant-a", WorkloadUID: "workload-uid",
		BudgetPolicyName: "budget", RoutingPolicyName: "routing", InputTokens: 10, InputTokensExact: true, MaxOutputTokens: 100}
	budget := aiopsv1alpha1.AIBudgetPolicy{ObjectMeta: metav1.ObjectMeta{Name: "budget", Namespace: "finance", UID: "budget-uid", Generation: 2}}
	routing := aiopsv1alpha1.AIRoutingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "routing", Namespace: "finance", UID: "routing-uid", Generation: 3}}
	candidate := approvalTestCandidate()
	approvalRequest := admissionApprovalRequest(req, budget, routing, candidate, metav1.NewTime(now.Add(time.Hour)))
	proposal := &aiopsv1alpha1.AIAdmissionApproval{ObjectMeta: metav1.ObjectMeta{Name: admissionApprovalName(req), Namespace: req.Namespace, UID: "proposal-uid", Generation: 1},
		Spec: aiopsv1alpha1.AIAdmissionApprovalSpec{Request: approvalRequest}, Status: aiopsv1alpha1.AIAdmissionApprovalStatus{
			ObservedGeneration: 1, Phase: aiopsv1alpha1.AIAdmissionApprovalPhaseApproved, RequestDigest: approvalRequest.RequestDigest,
			DecisionResourceUID: "decision-uid", DecisionResourceGeneration: 1, ApprovedAt: &metav1.Time{Time: now},
		}}
	c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(proposal).Build()
	srv := &server{k8s: c, auth: identityAuthenticator{now: func() time.Time { return now }}}
	if _, err := srv.validateAndConsumeAdmissionApproval(context.Background(), proposal.Name, req, budget, routing, []govar.Candidate{candidate}); err != nil {
		t.Fatalf("first consumption failed: %v", err)
	}
	// This models a crash after Lease creation but before ledger admission. A
	// retry fails closed instead of reusing the human authorization.
	if _, err := srv.validateAndConsumeAdmissionApproval(context.Background(), proposal.Name, req, budget, routing, []govar.Candidate{candidate}); !errors.Is(err, errApprovalReplayed) {
		t.Fatalf("post-crash retry err=%v, want replay rejection", err)
	}
}

func TestAdmissionApprovalRejectsMissingStaleCrossTenantAlteredAndExpired(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	req := govar.AdmitRequest{RequestID: "request-1", Namespace: "finance", TenantID: "tenant-a", WorkloadUID: "workload-uid", BudgetPolicyName: "budget", RoutingPolicyName: "routing", MaxOutputTokens: 100}
	budget := aiopsv1alpha1.AIBudgetPolicy{ObjectMeta: metav1.ObjectMeta{Name: "budget", Namespace: "finance", UID: "budget-uid", Generation: 2}}
	routing := aiopsv1alpha1.AIRoutingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "routing", Namespace: "finance", UID: "routing-uid", Generation: 3}}
	candidate := approvalTestCandidate()
	baseRequest := admissionApprovalRequest(req, budget, routing, candidate, metav1.NewTime(now.Add(time.Hour)))
	base := aiopsv1alpha1.AIAdmissionApproval{ObjectMeta: metav1.ObjectMeta{Name: admissionApprovalName(req), Namespace: req.Namespace, UID: "proposal-uid", Generation: 1}, Spec: aiopsv1alpha1.AIAdmissionApprovalSpec{Request: baseRequest},
		Status: aiopsv1alpha1.AIAdmissionApprovalStatus{ObservedGeneration: 1, Phase: aiopsv1alpha1.AIAdmissionApprovalPhaseApproved, RequestDigest: baseRequest.RequestDigest,
			DecisionResourceUID: "decision-uid", DecisionResourceGeneration: 1, ApprovedAt: &metav1.Time{Time: now}}}
	tests := []struct {
		name   string
		object *aiopsv1alpha1.AIAdmissionApproval
		ref    string
	}{
		{name: "missing", ref: base.Name},
		{name: "stale status", object: func() *aiopsv1alpha1.AIAdmissionApproval {
			v := base.DeepCopy()
			v.Status.ObservedGeneration = 0
			return v
		}(), ref: base.Name},
		{name: "cross tenant request", object: func() *aiopsv1alpha1.AIAdmissionApproval {
			v := base.DeepCopy()
			v.Spec.Request.TenantID = "tenant-b"
			v.Spec.Request.RequestDigest = v.Spec.Request.ComputeDigest()
			v.Status.RequestDigest = v.Spec.Request.RequestDigest
			return v
		}(), ref: base.Name},
		{name: "altered request", object: func() *aiopsv1alpha1.AIAdmissionApproval {
			v := base.DeepCopy()
			v.Spec.Request.MaxOutputTokens++
			return v
		}(), ref: base.Name},
		{name: "expired", object: func() *aiopsv1alpha1.AIAdmissionApproval {
			v := base.DeepCopy()
			v.Spec.Request.ExpiresAt = metav1.NewTime(now.Add(-time.Second))
			v.Spec.Request.RequestDigest = v.Spec.Request.ComputeDigest()
			v.Status.RequestDigest = v.Spec.Request.RequestDigest
			return v
		}(), ref: base.Name},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := fakeclient.NewClientBuilder().WithScheme(scheme)
			if test.object != nil {
				builder = builder.WithObjects(test.object)
			}
			srv := &server{k8s: builder.Build(), auth: identityAuthenticator{now: func() time.Time { return now }}}
			if _, err := srv.validateAndConsumeAdmissionApproval(context.Background(), test.ref, req, budget, routing, []govar.Candidate{candidate}); err == nil {
				t.Fatal("invalid approval was accepted")
			}
		})
	}
}

func approvalTestCandidate() govar.Candidate {
	snapshot := govar.RouteSnapshot{Namespace: "finance", ModelName: "model", ModelUID: "model-uid", ModelGeneration: 1, ModelResourceVersion: "model-rv",
		ProviderName: "provider", ProviderUID: "provider-uid", ProviderGeneration: 1, ProviderResourceVersion: "provider-rv", PricingVersion: "pricing-v1",
		PricingComplianceHash: strings.Repeat("a", 64), RouteBindingName: "primary", ProviderDeployment: "provider-model", Cluster: "backend", Authority: "backend.example", PathMode: "openai-body"}
	snapshot.SnapshotHash = govar.RouteSnapshotHash(snapshot)
	return govar.Candidate{ModelRef: "model", ProviderRef: "provider", ProviderType: "openai", PricingVersion: "pricing-v1", SnapshotVersion: snapshot.SnapshotHash, RouteSnapshot: snapshot, Feasible: true}
}

func TestWriteAPIErrorUsesStableReasonCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeAPIError(recorder, http.StatusConflict, "invalid_transition", fmt.Errorf("conflict"))
	if recorder.Code != http.StatusConflict || !bytes.Contains(recorder.Body.Bytes(), []byte(`"reason_code":"invalid_transition"`)) {
		t.Fatalf("response=%d %s", recorder.Code, recorder.Body.String())
	}
}

func signedRequest(t *testing.T, secret []byte, timestamp time.Time, path, tenant, workload string, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	ts := fmt.Sprintf("%d", timestamp.Unix())
	digest := sha256.Sum256(body)
	message := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n%x", ts, req.Method, req.URL.EscapedPath(), tenant, workload, "finance", digest)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(message))
	req.Header.Set("X-GOVAR-Tenant-ID", tenant)
	req.Header.Set("X-GOVAR-Workload-UID", workload)
	req.Header.Set("X-GOVAR-Namespace", "finance")
	req.Header.Set("X-GOVAR-Timestamp", ts)
	req.Header.Set("X-GOVAR-Signature", fmt.Sprintf("%x", mac.Sum(nil)))
	return req
}
