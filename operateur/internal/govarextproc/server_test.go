package govarextproc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
)

func TestParseUsageNormalizesCachedAndReasoningWithoutDoubleCounting(t *testing.T) {
	start := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	required := []aiopsv1alpha1.ProviderBillableBasis{aiopsv1alpha1.ProviderBasisInputTokens, aiopsv1alpha1.ProviderBasisCachedInputTokens, aiopsv1alpha1.ProviderBasisOutputTokens, aiopsv1alpha1.ProviderBasisReasoningTokens, aiopsv1alpha1.ProviderBasisRequest, aiopsv1alpha1.ProviderBasisToolCall, aiopsv1alpha1.ProviderBasisBillableSecond, aiopsv1alpha1.ProviderBasisCancellation, aiopsv1alpha1.ProviderBasisRetryAttempt, aiopsv1alpha1.ProviderBasisMediaUnit}
	body := []byte(`{"usage":{"prompt_tokens":100,"completion_tokens":80,"prompt_tokens_details":{"cached_tokens":40},"completion_tokens_details":{"reasoning_tokens":30}},"choices":[{"message":{"tool_calls":[{},{}]}}]}`)
	got, ok := parseUsage(body, required, start, start.Add(1500*time.Millisecond))
	if !ok {
		t.Fatal("valid provider usage rejected")
	}
	want := map[aiopsv1alpha1.ProviderBillableBasis]int64{aiopsv1alpha1.ProviderBasisInputTokens: 60, aiopsv1alpha1.ProviderBasisCachedInputTokens: 40, aiopsv1alpha1.ProviderBasisOutputTokens: 50, aiopsv1alpha1.ProviderBasisReasoningTokens: 30, aiopsv1alpha1.ProviderBasisRequest: 1, aiopsv1alpha1.ProviderBasisToolCall: 2, aiopsv1alpha1.ProviderBasisBillableSecond: 2, aiopsv1alpha1.ProviderBasisCancellation: 0, aiopsv1alpha1.ProviderBasisRetryAttempt: 0}
	if len(got.Quantities) != len(want) {
		t.Fatalf("quantities=%+v", got.Quantities)
	}
	for _, q := range got.Quantities {
		if want[q.Basis] != q.Quantity {
			t.Fatalf("%s=%d want=%d", q.Basis, q.Quantity, want[q.Basis])
		}
		delete(want, q.Basis)
	}
	if len(want) != 0 {
		t.Fatalf("missing normalized values: %v", want)
	}
}

func TestEnvoyExtProcLifecycleAdmitClaimRouteDeliverSettle(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithSpanProcessor(spanRecorder))
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		_ = tracerProvider.Shutdown(context.Background())
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})
	var calls []string
	admission := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/v1/admit":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["tenant_id"] != "tenant-a" || body["workload_uid"] != "uid-a" {
				t.Fatalf("untrusted request headers selected identity: %+v", body)
			}
			snapshot := extProcTestSnapshot("model-eu", "provider-model-eu", "backend-eu", "eu.provider.test")
			_ = json.NewEncoder(w).Encode(map[string]any{"decision": "ADMIT", "reason_code": "highest_utility_feasible", "selected_deployment": "model-eu", "provider_attempt_id": "attempt-1", "pricing_version": snapshot.PricingVersion, "route_snapshot": snapshot})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"reason_code": "ok"})
		}
	}))
	defer admission.Close()

	listener := bufconn.Listen(1 << 20)
	serverCredentials, clientCredentials := extProcTestCredentials(t, "spiffe://govar.local/gateway/envoy")
	grpcServer := grpc.NewServer(grpc.Creds(serverCredentials))
	identity := "spiffe://govar.local/ns/finance/pod/uid-a"
	extprocv3.RegisterExternalProcessorServer(grpcServer, &Server{AdmissionURL: admission.URL, MasterSecret: []byte("0123456789abcdef0123456789abcdef"), PrincipalRegistry: map[string]RouteBinding{
		identity: {Namespace: "finance", TenantID: "tenant-a", WorkloadUID: "uid-a", Team: "treasury", Application: "assistant", BudgetPolicy: "budget", RoutingPolicy: "routing"},
	}, AllowedGatewayURIs: map[string]struct{}{"spiffe://govar.local/gateway/envoy": {}}})
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()
	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(clientCredentials))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	stream, err := extprocv3.NewExternalProcessorClient(conn).Process(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	const parentTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	headers := &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":path", Value: "/v1/chat/completions?api-version=2026-01-01"}, {Key: "x-request-id", Value: "request-1"}, {Key: "x-forwarded-client-cert", Value: "By=spiffe://govar.local/gateway/envoy;Hash=" + strings.Repeat("ab", 32) + ";URI=" + identity},
		{Key: "traceparent", Value: "00-" + parentTraceID + "-00f067aa0ba902b7-01"},
		{Key: "x-govar-namespace", Value: "victim-ns"}, {Key: "x-govar-tenant-id", Value: "victim"},
		{Key: "x-govar-workload-uid", Value: "victim-uid"}, {Key: "x-govar-budget-policy", Value: "victim-budget"},
	}}
	if err := stream.Send(&extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_RequestHeaders{RequestHeaders: &extprocv3.HttpHeaders{Headers: headers}}}); err != nil {
		t.Fatal(err)
	}
	if response, err := stream.Recv(); err != nil || response.GetRequestHeaders() == nil {
		t.Fatalf("request headers response=%+v err=%v", response, err)
	} else {
		traceparent := headerMutationValue(response.GetRequestHeaders().GetResponse().GetHeaderMutation(), "traceparent")
		if !strings.HasPrefix(traceparent, "00-"+parentTraceID+"-") || strings.HasSuffix(traceparent, "-00f067aa0ba902b7-01") {
			t.Fatalf("provider-bound traceparent does not join the caller trace with a child span: %q", traceparent)
		}
	}
	requestBody := []byte(`{"model":"original","max_tokens":32,"messages":[{"role":"user","content":"public"}]}`)
	if err := stream.Send(&extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_RequestBody{RequestBody: &extprocv3.HttpBody{Body: requestBody, EndOfStream: true}}}); err != nil {
		t.Fatal(err)
	}
	bodyResponse, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	mutation := bodyResponse.GetRequestBody().GetResponse().GetHeaderMutation()
	if mutation == nil || headerMutationValue(mutation, "x-ai-eg-model") != "model-eu" || headerMutationValue(mutation, "x-govar-upstream-cluster") != "backend-eu" ||
		headerMutationValue(mutation, "x-govar-upstream-authority") != "eu.provider.test" || headerMutationValue(mutation, ":path") != "/v1/chat/completions?api-version=2026-01-01" {
		t.Fatalf("route mutation=%+v", mutation)
	}
	providerTraceparent := headerMutationValue(mutation, "traceparent")
	if !strings.HasPrefix(providerTraceparent, "00-"+parentTraceID+"-") {
		t.Fatalf("selected provider did not receive the joined provider-attempt context: %q", providerTraceparent)
	}
	var mutatedBody map[string]any
	if err := json.Unmarshal(bodyResponse.GetRequestBody().GetResponse().GetBodyMutation().GetBody(), &mutatedBody); err != nil || mutatedBody["model"] != "provider-model-eu" {
		t.Fatalf("body mutation=%+v err=%v", mutatedBody, err)
	}
	if err := stream.Send(&extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_ResponseHeaders{ResponseHeaders: &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}
	responseBody := []byte(`{"usage":{"prompt_tokens":10,"completion_tokens":20}}`)
	if err := stream.Send(&extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_ResponseBody{ResponseBody: &extprocv3.HttpBody{Body: responseBody, EndOfStream: true}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"/v1/admit", "/v1/dispatch", "/v1/dispatch", "/v1/settle"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatal(err)
	}
	_, _ = stream.Recv()
	wantSpans := map[string]bool{
		"govar.ext_proc.request": false, "govar.admission_request": false,
		"govar.route_actuation": false, "govar.dispatch_transition": false,
		"govar.provider_attempt": false, "govar.settlement": false,
	}
	for _, span := range spanRecorder.Ended() {
		if span.SpanContext().TraceID().String() != parentTraceID {
			t.Fatalf("span %q escaped the joined trace: %s", span.Name(), span.SpanContext().TraceID())
		}
		if _, required := wantSpans[span.Name()]; required {
			wantSpans[span.Name()] = true
		}
		if span.Name() == "govar.provider_attempt" && !strings.Contains(providerTraceparent, "-"+span.SpanContext().SpanID().String()+"-") {
			t.Errorf("backend traceparent %q is not the recorded provider-attempt span %s", providerTraceparent, span.SpanContext().SpanID())
		}
	}
	for name, observed := range wantSpans {
		if !observed {
			t.Errorf("joined trace missing span %q; ended=%d", name, len(spanRecorder.Ended()))
		}
	}
}

func extProcTestSnapshot(model, deployment, cluster, authority string) govar.RouteSnapshot {
	snapshot := govar.RouteSnapshot{Namespace: "finance", ModelName: model, ModelUID: "model-uid-1", ModelGeneration: 1, ModelResourceVersion: "model-rv-1",
		ProviderName: "provider", ProviderUID: "provider-uid-1", ProviderGeneration: 1, ProviderResourceVersion: "provider-rv-1",
		PricingVersion: "pricing-v1", PricingComplianceHash: strings.Repeat("a", 64), RouteBindingName: "primary",
		ProviderDeployment: deployment, Cluster: cluster, Authority: authority, PathMode: "openai-body"}
	snapshot.SnapshotHash = govar.RouteSnapshotHash(snapshot)
	return snapshot
}

func TestRouteAdaptersRewriteBodyAndPathFailClosed(t *testing.T) {
	tests := []struct {
		name, originalPath, mode, deployment, wantPath string
		wantModel                                      any
	}{
		{name: "openai responses", originalPath: "/v1/responses?trace=1", mode: "openai-body", deployment: "gpt-4.1", wantPath: "/v1/responses?trace=1", wantModel: "gpt-4.1"},
		{name: "azure deployment path", originalPath: "/openai/deployments/client-choice/responses?api-version=2026-01-01", mode: "azure-deployment-path", deployment: "approved-deployment", wantPath: "/openai/deployments/approved-deployment/responses?api-version=2026-01-01", wantModel: nil},
		{name: "google generate path", originalPath: "/v1beta/models/client-choice:generateContent?key=public", mode: "google-generate-path", deployment: "gemini-approved", wantPath: "/v1beta/models/gemini-approved:generateContent?key=public", wantModel: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := RouteTarget{SelectedModel: "catalog-model", ProviderDeployment: test.deployment, Cluster: "approved-cluster", Authority: "provider.test", PathMode: test.mode}
			body, err := rewriteRequestForRoute([]byte(`{"model":"client-choice","messages":[]}`), test.originalPath, target)
			if err != nil {
				t.Fatal(err)
			}
			path, err := routePath(test.originalPath, target)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			if path != test.wantPath || payload["model"] != test.wantModel {
				t.Fatalf("path=%q payload=%+v", path, payload)
			}
		})
	}
	if _, err := rewriteRequestForRoute([]byte(`{"model":"x"}`), "/v1/chat/completions", RouteTarget{PathMode: "caller-selected"}); err == nil {
		t.Fatal("unknown provider adapter was inferred instead of rejected")
	}
}

func TestAdmissionRouteSnapshotTamperingFailsBeforeClaim(t *testing.T) {
	base := extProcTestSnapshot("model-eu", "provider-model-eu", "backend-eu", "eu.provider.test")
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "missing snapshot", mutate: func(value map[string]any) { delete(value, "route_snapshot") }},
		{name: "route field changed", mutate: func(value map[string]any) { value["route_snapshot"].(map[string]any)["cluster"] = "attacker" }},
		{name: "snapshot hash changed", mutate: func(value map[string]any) {
			value["route_snapshot"].(map[string]any)["snapshot_hash"] = strings.Repeat("f", 64)
		}},
		{name: "model cross field", mutate: func(value map[string]any) { value["selected_deployment"] = "other" }},
		{name: "pricing cross field", mutate: func(value map[string]any) { value["pricing_version"] = "other" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls []string
			admission := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.URL.Path)
				raw, _ := json.Marshal(base)
				var snapshot map[string]any
				_ = json.Unmarshal(raw, &snapshot)
				value := map[string]any{"decision": "ADMIT", "reason_code": "highest_utility_feasible", "selected_deployment": base.ModelName, "provider_attempt_id": "attempt", "pricing_version": base.PricingVersion, "route_snapshot": snapshot}
				test.mutate(value)
				_ = json.NewEncoder(w).Encode(value)
			}))
			defer admission.Close()
			server := &Server{AdmissionURL: admission.URL, MasterSecret: []byte("0123456789abcdef0123456789abcdef")}
			state := &streamState{headers: map[string]string{"x-request-id": "tamper", ":path": "/v1/chat/completions"}, requestBody: []byte(`{"model":"client","max_tokens":10}`), binding: RouteBinding{Namespace: "finance", TenantID: "tenant", WorkloadUID: "uid"}}
			if _, err := server.admitAndClaim(context.Background(), state); err == nil {
				t.Fatal("tampered snapshot reached claim")
			}
			if !reflect.DeepEqual(calls, []string{"/v1/admit"}) {
				t.Fatalf("calls=%v", calls)
			}
		})
	}
}

func TestXFCCMustConformToAuthenticatedGatewaySANITIZESet(t *testing.T) {
	server := &Server{PrincipalRegistry: map[string]RouteBinding{"spiffe://govar.local/ns/finance/pod/uid-a": {Namespace: "finance", TenantID: "tenant-a", WorkloadUID: "uid-a", BudgetPolicy: "budget", RoutingPolicy: "routing"}}}
	for _, forged := range []string{
		"URI=spiffe://govar.local/ns/finance/pod/uid-a",
		"By=spiffe://govar.local/gateway/other;Hash=" + strings.Repeat("ab", 32) + ";URI=spiffe://govar.local/ns/finance/pod/uid-a",
		"By=spiffe://govar.local/gateway/envoy;Hash=not-a-sha256;URI=spiffe://govar.local/ns/finance/pod/uid-a",
	} {
		if _, err := server.resolvePrincipal(context.Background(), "spiffe://govar.local/gateway/envoy", forged); err == nil {
			t.Fatalf("forged/nonconforming XFCC accepted: %s", forged)
		}
	}
}

func headerMutationValue(mutation *extprocv3.HeaderMutation, key string) string {
	for _, option := range mutation.SetHeaders {
		if option.Header != nil && option.Header.Key == key {
			return first(option.Header.Value, string(option.Header.RawValue))
		}
	}
	return ""
}

func TestExtProcRejectsUnauthenticatedDirectGRPC(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	serverCredentials, _ := extProcTestCredentials(t, "spiffe://govar.local/gateway/envoy")
	server := grpc.NewServer(grpc.Creds(serverCredentials))
	extprocv3.RegisterExternalProcessorServer(server, &Server{AllowedGatewayURIs: map[string]struct{}{"spiffe://govar.local/gateway/envoy": {}}})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err == nil {
		defer conn.Close()
		stream, streamErr := extprocv3.NewExternalProcessorClient(conn).Process(ctx)
		if streamErr == nil {
			streamErr = stream.Send(&extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_RequestHeaders{RequestHeaders: &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{Key: "x-forwarded-client-cert", Value: "URI=spiffe://govar.local/ns/victim/pod/forged"}}}}}})
		}
		if streamErr == nil {
			_, streamErr = stream.Recv()
		}
		if streamErr == nil {
			t.Fatal("unauthenticated direct ext_proc gRPC call was accepted")
		}
	}
}

func TestExtProcRejectsValidCertificateWithUnapprovedGatewayIdentity(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	serverCredentials, attackerCredentials := extProcTestCredentials(t, "spiffe://govar.local/gateway/attacker")
	server := grpc.NewServer(grpc.Creds(serverCredentials))
	extprocv3.RegisterExternalProcessorServer(server, &Server{AllowedGatewayURIs: map[string]struct{}{"spiffe://govar.local/gateway/envoy": {}}})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(attackerCredentials))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	stream, err := extprocv3.NewExternalProcessorClient(conn).Process(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sendErr := stream.Send(&extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_RequestHeaders{RequestHeaders: &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{Key: "x-forwarded-client-cert", Value: "URI=spiffe://govar.local/ns/victim/pod/forged"}}}}}})
	if sendErr == nil {
		_, sendErr = stream.Recv()
	}
	if sendErr == nil {
		t.Fatal("unapproved mTLS gateway identity was accepted")
	}
}

func extProcTestCredentials(t *testing.T, clientURI string) (credentials.TransportCredentials, credentials.TransportCredentials) {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	issue := func(serial int64, server bool, identity string) tls.Certificate {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "govar-test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment}
		if server {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			template.DNSNames = []string{"bufnet"}
		} else {
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
			parsed, parseErr := url.Parse(identity)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			template.URIs = []*url.URL{parsed}
		}
		der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		certificate, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatal(err)
		}
		return certificate
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	serverCertificate := issue(2, true, "")
	clientCertificate := issue(3, false, clientURI)
	serverTLS := credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{serverCertificate}, ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS12})
	clientTLS := credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{clientCertificate}, RootCAs: pool, ServerName: "bufnet", MinVersion: tls.VersionTLS12})
	return serverTLS, clientTLS
}
