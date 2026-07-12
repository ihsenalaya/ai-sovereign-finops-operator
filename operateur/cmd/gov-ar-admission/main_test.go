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
	"testing"
	"time"

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
}

func (b *countingBackend) Dispatch(req govar.DispatchRequest) (govar.Reservation, govar.ReasonCode, error) {
	b.dispatchCalls++
	return b.Engine.Dispatch(req)
}

func TestBoundPodCannotMutateAnotherTenantWithForgedHeaderOrBody(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "finance", UID: types.UID("uid-a"),
		Annotations: map[string]string{podinjector.GOVARTenantKey: "tenant-a", podinjector.GOVARBudgetPolicyKey: "budget", podinjector.GOVARRoutingKey: "routing"}}}
	backend := &countingBackend{Engine: govar.NewEngine()}
	srv := &server{k8s: fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build(), engine: backend,
		auth: identityAuthenticator{reviewToken: func(context.Context, string) (authenticatedPrincipal, error) {
			return authenticatedPrincipal{namespace: "finance", workloadUID: "uid-a", podName: "agent"}, nil
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
		Labels: map[string]string{"aiops.imperium.io/team": "treasury", "app": "assistant"}, Annotations: map[string]string{
			"aiops.imperium.io/govar-tenant": "tenant-a", "aiops.imperium.io/govar-budget-policy": "budget",
			"aiops.imperium.io/govar-routing-policy": "routing", "aiops.imperium.io/govar-sensitive-data": "true",
			"aiops.imperium.io/govar-allowed-zones": "EU,francecentral"}}}
	srv := &server{k8s: fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()}
	trusted, err := srv.resolveTrustedWorkload(context.Background(), authenticatedPrincipal{namespace: "finance", tenantID: "tenant-a", workloadUID: "uid-a"})
	if err != nil || trusted.resourceVersion != "7" || trusted.team != "treasury" || !trusted.sensitive {
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
