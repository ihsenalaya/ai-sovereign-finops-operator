package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(aiopsv1alpha1.AddToScheme(scheme))
}

type server struct {
	k8s    client.Client
	engine admissionBackend
}

type admissionBackend interface {
	Admit(req govar.AdmitRequest, budget aiopsv1alpha1.AIBudgetPolicy, routing aiopsv1alpha1.AIRoutingPolicy, candidates []govar.Candidate) (govar.AdmitResponse, error)
	Settle(req govar.SettleRequest) (govar.Reservation, govar.ReasonCode, error)
	Cancel(req govar.CancelRequest) (govar.Reservation, error)
	Liability(tenantID string) govar.LiabilityResponse
}

func main() {
	addr := os.Getenv("GOV_AR_ADMISSION_ADDR")
	if addr == "" {
		addr = ":8084"
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
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		pgEngine, err := govar.NewPostgresEngine(context.Background(), databaseURL)
		if err != nil {
			log.Fatalf("create postgres engine: %v", err)
		}
		defer pgEngine.Close()
		engine = pgEngine
		log.Println("gov-ar-admission using PostgreSQL-backed ledger")
	} else {
		engine = govar.NewEngine()
		log.Println("gov-ar-admission using in-memory ledger")
	}

	srv := &server{k8s: k8sClient, engine: engine}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/readyz", handleReadyz)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/v1/admit", srv.handleAdmit)
	mux.HandleFunc("/v1/settle", srv.handleSettle)
	mux.HandleFunc("/v1/cancel", srv.handleCancel)
	mux.HandleFunc("/v1/liability/", srv.handleLiability)

	log.Printf("gov-ar-admission listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *server) handleAdmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req govar.AdmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, resp)
}

func (s *server) handleSettle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req govar.SettleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, code, err := s.engine.Settle(req)
	if err != nil && code == "" {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"request_id":     res.RequestID,
		"tenant_id":      res.TenantID,
		"reason_code":    code,
		"settled":        res.Settled,
		"actual_cost":    res.ActualCost,
		"pricingVersion": res.PricingVersion,
	})
}

func (s *server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req govar.CancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := s.engine.Cancel(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"request_id":  res.RequestID,
		"tenant_id":   res.TenantID,
		"reason_code": govar.ReasonCanceled,
		"canceled":    res.Canceled,
	})
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
	writeJSON(w, s.engine.Liability(tenantID))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
