package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
	"github.com/imperium/ai-sovereign-finops-operator/internal/govarextproc"
	"github.com/imperium/ai-sovereign-finops-operator/internal/webhook/podinjector"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(aiopsv1alpha1.AddToScheme(scheme))
}

type server struct {
	k8s    client.Client
	engine admissionBackend
	auth   identityAuthenticator
}

type admissionBackend interface {
	Admit(req govar.AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy, candidates []govar.Candidate) (govar.AdmitResponse, error)
	Dispatch(req govar.DispatchRequest) (govar.Reservation, govar.ReasonCode, error)
	Settle(req govar.SettleRequest) (govar.Reservation, govar.ReasonCode, error)
	Cancel(req govar.CancelRequest) (govar.Reservation, govar.ReasonCode, error)
	LiabilityWithError(tenantID string) (govar.LiabilityResponse, error)
	Ready(context.Context) error
}

type authenticatedPrincipal struct {
	tenantID    string
	workloadUID string
	namespace   string
	podName     string
}

type identityAuthenticator struct {
	masterSecret []byte
	now          func() time.Time
	reviewToken  func(context.Context, string) (authenticatedPrincipal, error)
}

func main() {
	addr := os.Getenv("GOV_AR_ADMISSION_ADDR")
	if addr == "" {
		addr = ":8084"
	}
	identitySecret := os.Getenv("GOV_AR_IDENTITY_MASTER_SECRET")
	if len(identitySecret) < 32 {
		log.Fatal("GOV_AR_IDENTITY_MASTER_SECRET must contain at least 32 bytes")
	}

	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Fatalf("get kubeconfig: %v", err)
	}
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("create k8s client: %v", err)
	}

	var engine admissionBackend
	devInMemory := strings.EqualFold(strings.TrimSpace(os.Getenv("GOV_AR_DEV_IN_MEMORY")), "true")
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		pgEngine, err := govar.NewPostgresEngine(context.Background(), databaseURL)
		if err != nil {
			log.Fatalf("create postgres engine: %v", err)
		}
		defer pgEngine.Close()
		engine = pgEngine
		log.Println("gov-ar-admission using PostgreSQL-backed ledger")
	} else {
		if !devInMemory {
			log.Fatal("DATABASE_URL is required unless GOV_AR_DEV_IN_MEMORY=true")
		}
		engine = govar.NewEngine()
		log.Println("gov-ar-admission using explicit single-replica development in-memory ledger")
	}

	srv := &server{k8s: k8sClient, engine: engine, auth: identityAuthenticator{masterSecret: []byte(identitySecret), now: time.Now, reviewToken: tokenReviewFunc(k8sClient)}}
	extProcAddr := strings.TrimSpace(os.Getenv("GOV_AR_EXT_PROC_ADDR"))
	if extProcAddr == "" {
		extProcAddr = ":9002"
	}
	extListener, err := net.Listen("tcp", extProcAddr)
	if err != nil {
		log.Fatalf("listen ext_proc: %v", err)
	}
	allowInsecureExtProc := devInMemory && strings.EqualFold(strings.TrimSpace(os.Getenv("GOV_AR_EXT_PROC_DEV_INSECURE")), "true")
	grpcOptions := make([]grpc.ServerOption, 0, 1)
	if !allowInsecureExtProc {
		transportCredentials, err := extProcServerCredentials(
			os.Getenv("GOV_AR_EXT_PROC_TLS_CERT_FILE"),
			os.Getenv("GOV_AR_EXT_PROC_TLS_KEY_FILE"),
			os.Getenv("GOV_AR_EXT_PROC_CLIENT_CA_FILE"),
		)
		if err != nil {
			log.Fatalf("configure ext_proc mTLS: %v", err)
		}
		grpcOptions = append(grpcOptions, grpc.Creds(transportCredentials))
	}
	extGRPC := grpc.NewServer(grpcOptions...)
	admissionURL := strings.TrimSpace(os.Getenv("GOV_AR_INTERNAL_ADMISSION_URL"))
	if admissionURL == "" {
		admissionURL = "http://127.0.0.1:8084"
	}
	gatewaySPIFFEID := strings.TrimSpace(os.Getenv("GOV_AR_EXT_PROC_GATEWAY_SPIFFE_ID"))
	if !allowInsecureExtProc && gatewaySPIFFEID == "" {
		log.Fatal("GOV_AR_EXT_PROC_GATEWAY_SPIFFE_ID is required for ext_proc mTLS")
	}
	extprocv3.RegisterExternalProcessorServer(extGRPC, &govarextproc.Server{
		AdmissionURL: admissionURL, MasterSecret: []byte(identitySecret), ResolvePrincipal: extProcPrincipalResolver(srv),
		AllowedGatewayURIs: map[string]struct{}{gatewaySPIFFEID: {}}, AllowInsecureDev: allowInsecureExtProc,
	})
	go func() {
		if err := extGRPC.Serve(extListener); err != nil {
			log.Printf("ext_proc server stopped: %v", err)
		}
	}()
	defer extGRPC.GracefulStop()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/readyz", srv.handleReadyz)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/v1/admit", srv.handleAdmit)
	mux.HandleFunc("/v1/dispatch", srv.handleDispatch)
	mux.HandleFunc("/v1/settle", srv.handleSettle)
	mux.HandleFunc("/v1/cancel", srv.handleCancel)
	mux.HandleFunc("/v1/liability/", srv.handleLiability)

	log.Printf("gov-ar-admission listening on %s; Envoy ext_proc on %s", addr, extProcAddr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func extProcServerCredentials(certFile, keyFile, clientCAFile string) (credentials.TransportCredentials, error) {
	if strings.TrimSpace(certFile) == "" || strings.TrimSpace(keyFile) == "" || strings.TrimSpace(clientCAFile) == "" {
		return nil, errors.New("server certificate, private key, and client CA files are required")
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load ext_proc server keypair: %w", err)
	}
	caPEM, err := os.ReadFile(clientCAFile)
	if err != nil {
		return nil, fmt.Errorf("read ext_proc client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("ext_proc client CA contains no certificates")
	}
	return credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{certificate}, ClientCAs: clientCAs, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS13}), nil
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.engine.Ready(ctx); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, govar.ReasonInvalidTransition, fmt.Errorf("ledger not ready: %w", err))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *server) handleAdmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	principal, body, err := s.auth.authenticate(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, govar.ReasonPrincipalMismatch, err)
		return
	}
	var req govar.AdmitRequest
	if err := decodeStrictJSON(body, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, govar.ReasonInvalidTransition, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	trusted, err := s.resolveTrustedWorkload(ctx, principal)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, govar.ReasonPrincipalMismatch, err)
		return
	}
	if err := bindTrustedAdmitRequest(&req, trusted); err != nil {
		writeAPIError(w, http.StatusForbidden, govar.ReasonPrincipalMismatch, err)
		return
	}
	req.AuthenticatedTenantID = trusted.tenant
	req.AuthenticatedWorkloadUID = trusted.uid
	req.AuthenticatedNamespace = trusted.namespace

	var budget aiopsv1alpha1.AIBudgetPolicy
	if err := s.k8s.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.BudgetPolicyName}, &budget); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var routing aiopsv1alpha1.AIRoutingPolicy
	if err := s.k8s.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.RoutingPolicyName}, &routing); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var modelList aiopsv1alpha1.AIModelList
	if err := s.k8s.List(ctx, &modelList, client.InNamespace(req.Namespace)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var providerList aiopsv1alpha1.AIProviderList
	if err := s.k8s.List(ctx, &providerList, client.InNamespace(req.Namespace)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	providers := map[string]aiopsv1alpha1.AIProvider{}
	for i := range providerList.Items {
		providers[providerList.Items[i].Name] = providerList.Items[i]
	}

	candidates := govar.BuildCandidates(govar.RequestContext{
		Namespace:     req.Namespace,
		Team:          req.Team,
		Application:   req.Application,
		SensitiveData: req.SensitiveData,
		AllowedZones:  req.AllowedZones,
	}, modelList.Items, providers)
	resp, err := s.engine.Admit(req, budget, routing, candidates)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, govar.ReasonInvalidTransition, err)
		return
	}
	writeJSON(w, resp)
}

func (s *server) handleDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, body, err := s.auth.authenticate(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, govar.ReasonPrincipalMismatch, err)
		return
	}
	var req govar.DispatchRequest
	if err := decodeStrictJSON(body, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, govar.ReasonInvalidTransition, err)
		return
	}
	trusted, err := s.authorizeEvent(r.Context(), principal, req.TenantID, req.WorkloadUID)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, govar.ReasonPrincipalMismatch, err)
		return
	}
	req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID = trusted.tenant, trusted.uid
	res, code, err := s.engine.Dispatch(req)
	if err != nil {
		writeAPIError(w, http.StatusConflict, code, err)
		return
	}
	writeJSON(w, transitionResponse(res, code))
}

func (s *server) handleSettle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, body, err := s.auth.authenticate(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, govar.ReasonPrincipalMismatch, err)
		return
	}
	var req govar.SettleRequest
	if err := decodeStrictJSON(body, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, govar.ReasonInvalidTransition, err)
		return
	}
	if req.ActualCostMicros != 0 {
		writeAPIError(w, http.StatusForbidden, govar.ReasonPrincipalMismatch, errors.New("gateway settlement must derive cost from authoritative usage tokens"))
		return
	}
	trusted, err := s.authorizeEvent(r.Context(), principal, req.TenantID, req.WorkloadUID)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, govar.ReasonPrincipalMismatch, err)
		return
	}
	req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID = trusted.tenant, trusted.uid
	res, code, err := s.engine.Settle(req)
	if err != nil {
		writeAPIError(w, http.StatusConflict, code, err)
		return
	}
	writeJSON(w, transitionResponse(res, code))
}

func (s *server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	principal, body, err := s.auth.authenticate(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, govar.ReasonPrincipalMismatch, err)
		return
	}
	var req govar.CancelRequest
	if err := decodeStrictJSON(body, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, govar.ReasonInvalidTransition, err)
		return
	}
	if req.AuthoritativeUnbilled {
		writeAPIError(w, http.StatusForbidden, govar.ReasonPrincipalMismatch, errors.New("gateway authority cannot release liability as authoritative unbilled"))
		return
	}
	trusted, err := s.authorizeEvent(r.Context(), principal, req.TenantID, req.WorkloadUID)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, govar.ReasonPrincipalMismatch, err)
		return
	}
	req.AuthenticatedTenantID, req.AuthenticatedWorkloadUID = trusted.tenant, trusted.uid
	res, code, err := s.engine.Cancel(req)
	if err != nil {
		writeAPIError(w, http.StatusConflict, code, err)
		return
	}
	writeJSON(w, transitionResponse(res, code))
}

func (s *server) handleLiability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenantID := r.URL.Path[len("/v1/liability/"):]
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}
	principal, _, err := s.auth.authenticate(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, govar.ReasonPrincipalMismatch, err)
		return
	}
	trusted, err := s.resolveTrustedWorkload(r.Context(), principal)
	if err != nil {
		writeAPIError(w, http.StatusForbidden, govar.ReasonPrincipalMismatch, err)
		return
	}
	if trusted.tenant != tenantID {
		writeAPIError(w, http.StatusForbidden, govar.ReasonPrincipalMismatch, errors.New("authenticated tenant cannot read another tenant ledger"))
		return
	}
	liability, err := s.engine.LiabilityWithError(tenantID)
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, govar.ReasonInvalidTransition, fmt.Errorf("ledger query failed: %w", err))
		return
	}
	writeJSON(w, liability)
}

func transitionResponse(res govar.Reservation, code govar.ReasonCode) map[string]any {
	return map[string]any{
		"request_id": res.RequestID, "tenant_id": res.TenantID, "workload_uid": res.WorkloadUID,
		"provider_attempt_id": res.ProviderAttemptID, "reason_code": code, "state": res.State,
		"outbox_state": res.OutboxState, "reserved_cost_micros": res.ReservedCostMicros,
		"provisional_cost_micros": res.ProvisionalCostMicros,
		"residual_hold_micros":    res.ResidualHoldMicros, "usage_version": res.UsageVersion,
		"finalized": res.Finalized, "pricing_version": res.PricingVersion,
	}
}

func (a identityAuthenticator) authenticate(r *http.Request) (authenticatedPrincipal, []byte, error) {
	if authorization := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(authorization, "Bearer ") {
		if a.reviewToken == nil {
			return authenticatedPrincipal{}, nil, errors.New("projected token review is not configured")
		}
		principal, err := a.reviewToken(r.Context(), strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")))
		if err != nil {
			return authenticatedPrincipal{}, nil, err
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		return principal, body, err
	}
	if len(a.masterSecret) < 32 {
		return authenticatedPrincipal{}, nil, errors.New("identity authentication is not configured")
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-GOVAR-Tenant-ID"))
	workloadUID := strings.TrimSpace(r.Header.Get("X-GOVAR-Workload-UID"))
	namespace := strings.TrimSpace(r.Header.Get("X-GOVAR-Namespace"))
	timestamp := strings.TrimSpace(r.Header.Get("X-GOVAR-Timestamp"))
	signature := strings.TrimSpace(r.Header.Get("X-GOVAR-Signature"))
	if tenantID == "" || workloadUID == "" || namespace == "" || timestamp == "" || signature == "" {
		return authenticatedPrincipal{}, nil, errors.New("signed GOV-AR identity headers are required")
	}
	unixSeconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return authenticatedPrincipal{}, nil, errors.New("invalid GOV-AR identity timestamp")
	}
	now := time.Now()
	if a.now != nil {
		now = a.now()
	}
	if delta := now.Sub(time.Unix(unixSeconds, 0)); delta < -2*time.Minute || delta > 2*time.Minute {
		return authenticatedPrincipal{}, nil, errors.New("GOV-AR identity signature is outside the replay window")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return authenticatedPrincipal{}, nil, errors.New("read authenticated body")
	}
	digest := sha256.Sum256(body)
	message := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n%x", timestamp, r.Method, r.URL.EscapedPath(), tenantID, workloadUID, namespace, digest)
	signerKey := deriveSignerKey(a.masterSecret, namespace, tenantID, workloadUID)
	mac := hmac.New(sha256.New, signerKey)
	_, _ = mac.Write([]byte(message))
	want := fmt.Sprintf("%x", mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(want)) {
		return authenticatedPrincipal{}, nil, errors.New("invalid GOV-AR identity signature")
	}
	return authenticatedPrincipal{tenantID: tenantID, workloadUID: workloadUID, namespace: namespace}, body, nil
}

func tokenReviewFunc(k8s client.Client) func(context.Context, string) (authenticatedPrincipal, error) {
	return func(ctx context.Context, token string) (authenticatedPrincipal, error) {
		if token == "" {
			return authenticatedPrincipal{}, errors.New("projected service-account token is empty")
		}
		review := &authenticationv1.TokenReview{Spec: authenticationv1.TokenReviewSpec{Token: token, Audiences: []string{podinjector.GOVARTokenAudience}}}
		if err := k8s.Create(ctx, review); err != nil {
			return authenticatedPrincipal{}, fmt.Errorf("TokenReview failed: %w", err)
		}
		if !review.Status.Authenticated || !slices.Contains(review.Status.Audiences, podinjector.GOVARTokenAudience) {
			return authenticatedPrincipal{}, errors.New("projected token was not authenticated for GOV-AR audience")
		}
		uidValues := review.Status.User.Extra["authentication.kubernetes.io/pod-uid"]
		if len(uidValues) != 1 || strings.TrimSpace(uidValues[0]) == "" {
			return authenticatedPrincipal{}, errors.New("TokenReview lacks bound Pod UID")
		}
		parts := strings.Split(review.Status.User.Username, ":")
		if len(parts) != 4 || parts[0] != "system" || parts[1] != "serviceaccount" {
			return authenticatedPrincipal{}, errors.New("TokenReview user is not a service account")
		}
		podNames := review.Status.User.Extra["authentication.kubernetes.io/pod-name"]
		if len(podNames) != 1 || strings.TrimSpace(podNames[0]) == "" {
			return authenticatedPrincipal{}, errors.New("TokenReview lacks bound Pod name")
		}
		return authenticatedPrincipal{namespace: parts[2], workloadUID: strings.TrimSpace(uidValues[0]), podName: strings.TrimSpace(podNames[0])}, nil
	}
}

func deriveSignerKey(master []byte, namespace, tenantID, workloadUID string) []byte {
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte("govar-identity-v2\x00" + namespace + "\x00" + tenantID + "\x00" + workloadUID))
	return []byte(fmt.Sprintf("%x", mac.Sum(nil)))
}

type trustedWorkload struct {
	namespace, tenant, team, application, budgetPolicy, routingPolicy string
	uid, resourceVersion                                              string
	sensitive                                                         bool
	zones                                                             []string
}

func (s *server) resolveTrustedWorkload(ctx context.Context, principal authenticatedPrincipal) (trustedWorkload, error) {
	if principal.podName != "" {
		var pod corev1.Pod
		if err := s.k8s.Get(ctx, client.ObjectKey{Namespace: principal.namespace, Name: principal.podName}, &pod); err != nil {
			return trustedWorkload{}, fmt.Errorf("bound Pod lookup failed: %w", err)
		}
		if string(pod.UID) != principal.workloadUID {
			return trustedWorkload{}, errors.New("TokenReview Pod UID no longer matches bound Pod")
		}
		return trustedWorkloadFromPod(&pod), nil
	}
	var pods corev1.PodList
	if err := s.k8s.List(ctx, &pods, client.InNamespace(principal.namespace)); err != nil {
		return trustedWorkload{}, fmt.Errorf("list authenticated namespace pods: %w", err)
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if string(pod.UID) != principal.workloadUID {
			continue
		}
		return trustedWorkloadFromPod(pod), nil
	}
	return trustedWorkload{}, errors.New("signed workload UID does not exist in signed namespace")
}

func trustedWorkloadFromPod(pod *corev1.Pod) trustedWorkload {
	annotations, labels := pod.Annotations, pod.Labels
	application := strings.TrimSpace(annotations[podinjector.ApplicationKey])
	if application == "" {
		application = firstNonEmpty(labels["app.kubernetes.io/name"], labels["app"], labels["k8s-app"], pod.Name)
	}
	team := strings.TrimSpace(labels["aiops.imperium.io/team"])
	tenant := strings.TrimSpace(annotations[podinjector.GOVARTenantKey])
	if tenant == "" {
		tenant = firstNonEmpty(team, application)
	}
	return trustedWorkload{namespace: pod.Namespace, uid: string(pod.UID), resourceVersion: pod.ResourceVersion, tenant: tenant, team: team, application: application, budgetPolicy: strings.TrimSpace(annotations[podinjector.GOVARBudgetPolicyKey]), routingPolicy: strings.TrimSpace(annotations[podinjector.GOVARRoutingKey]), sensitive: strings.EqualFold(strings.TrimSpace(annotations[podinjector.GOVARSensitiveKey]), "true"), zones: splitAndNormalize(annotations[podinjector.GOVARZonesKey])}
}

func bindTrustedAdmitRequest(req *govar.AdmitRequest, trusted trustedWorkload) error {
	if req.Namespace != trusted.namespace || req.TenantID != trusted.tenant || req.WorkloadUID != trusted.uid || req.Team != trusted.team || req.Application != trusted.application ||
		req.BudgetPolicyName != trusted.budgetPolicy || req.RoutingPolicyName != trusted.routingPolicy || req.SensitiveData != trusted.sensitive ||
		!slices.Equal(sortedCopy(req.AllowedZones), trusted.zones) {
		return errors.New("admission body conflicts with trusted Pod metadata")
	}
	return nil
}

func (s *server) authorizeEvent(ctx context.Context, principal authenticatedPrincipal, tenantID, workloadUID string) (trustedWorkload, error) {
	trusted, err := s.resolveTrustedWorkload(ctx, principal)
	if err != nil {
		return trustedWorkload{}, err
	}
	if tenantID != trusted.tenant || workloadUID != trusted.uid {
		return trustedWorkload{}, errors.New("event identity conflicts with trusted live Pod metadata")
	}
	return trusted, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
func splitAndNormalize(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.ToLower(strings.TrimSpace(item)); item != "" {
			out = append(out, item)
		}
	}
	slices.Sort(out)
	return out
}
func sortedCopy(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			out = append(out, value)
		}
	}
	slices.Sort(out)
	return out
}

func extProcPrincipalResolver(srv *server) func(context.Context, string) (govarextproc.RouteBinding, error) {
	return func(ctx context.Context, identity string) (govarextproc.RouteBinding, error) {
		parsed, err := url.Parse(identity)
		if err != nil || parsed.Scheme != "spiffe" || parsed.Host != "govar.local" {
			return govarextproc.RouteBinding{}, errors.New("untrusted ext_proc SPIFFE identity")
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "ns" || parts[2] != "pod" || parts[1] == "" || parts[3] == "" {
			return govarextproc.RouteBinding{}, errors.New("invalid ext_proc SPIFFE workload path")
		}
		trusted, err := srv.resolveTrustedWorkload(ctx, authenticatedPrincipal{namespace: parts[1], workloadUID: parts[3]})
		if err != nil {
			return govarextproc.RouteBinding{}, err
		}
		return govarextproc.RouteBinding{Namespace: trusted.namespace, TenantID: trusted.tenant, WorkloadUID: trusted.uid, Team: trusted.team, Application: trusted.application, BudgetPolicy: trusted.budgetPolicy, RoutingPolicy: trusted.routingPolicy, Sensitive: trusted.sensitive, AllowedZones: trusted.zones}, nil
	}
}

func decodeStrictJSON(body []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, status int, code govar.ReasonCode, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"reason_code": code, "error": err.Error()})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
