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
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestEnvoyExtProcLifecycleAdmitClaimRouteDeliverSettle(t *testing.T) {
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
			_ = json.NewEncoder(w).Encode(map[string]any{"Decision": "ADMIT", "ReasonCode": "highest_utility_feasible", "SelectedDeployment": "model-eu", "ProviderAttemptID": "attempt-1"})
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

	headers := &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: "x-request-id", Value: "request-1"}, {Key: "x-forwarded-client-cert", Value: "By=spiffe://gateway;Hash=abc;URI=" + identity},
		{Key: "x-govar-namespace", Value: "victim-ns"}, {Key: "x-govar-tenant-id", Value: "victim"},
		{Key: "x-govar-workload-uid", Value: "victim-uid"}, {Key: "x-govar-budget-policy", Value: "victim-budget"},
	}}
	if err := stream.Send(&extprocv3.ProcessingRequest{Request: &extprocv3.ProcessingRequest_RequestHeaders{RequestHeaders: &extprocv3.HttpHeaders{Headers: headers}}}); err != nil {
		t.Fatal(err)
	}
	if response, err := stream.Recv(); err != nil || response.GetRequestHeaders() == nil {
		t.Fatalf("request headers response=%+v err=%v", response, err)
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
	if mutation == nil || mutation.SetHeaders[0].Header.Value != "model-eu" {
		t.Fatalf("route mutation=%+v", mutation)
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
