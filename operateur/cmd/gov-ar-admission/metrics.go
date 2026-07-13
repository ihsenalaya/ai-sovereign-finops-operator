package main

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
)

var (
	govarHTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "govar_http_requests_total",
		Help: "Authenticated GOV-AR API requests by bounded endpoint, method, and status class.",
	}, []string{"endpoint", "method", "status_class"})
	govarHTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "govar_http_request_duration_seconds",
		Help:    "GOV-AR API request duration by bounded endpoint and method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"endpoint", "method"})
	govarAdmissionDecisions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "govar_admission_decisions_total",
		Help: "GOV-AR admission decisions by closed decision and reason code.",
	}, []string{"decision", "reason"})
	govarLedgerTransitions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "govar_ledger_transitions_total",
		Help: "Reserve-settle ledger transition attempts by transition, reason, and result.",
	}, []string{"transition", "reason", "success"})
	govarReconciliationRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "govar_reconciliation_runs_total",
		Help: "Durable reconciliation worker runs by result.",
	}, []string{"result"})
	govarReconciledRecords = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "govar_reconciliation_records_total",
		Help: "Ledger records reconciled by the durable expiration worker.",
	})
	govarPendingReconciliation = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "govar_reconciliation_pending_records",
		Help: "Pending reconciliation records observed in the worker's bounded scan.",
	})
	govarReconciliationLastSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "govar_reconciliation_last_success_unixtime",
		Help: "Unix time of the last successful durable reconciliation pass.",
	})
)

func init() {
	prometheus.MustRegister(
		govarHTTPRequests,
		govarHTTPDuration,
		govarAdmissionDecisions,
		govarLedgerTransitions,
		govarReconciliationRuns,
		govarReconciledRecords,
		govarPendingReconciliation,
		govarReconciliationLastSuccess,
	)
}

type statusCapturingWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap lets net/http.ResponseController retain optional capabilities of the
// original writer without copying identity-bearing request data into metrics.
func (w *statusCapturingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusCapturingWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func instrumentHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		endpoint := boundedEndpoint(r.URL.Path)
		r, span := startHTTPServerSpan(r, endpoint)
		captured := &statusCapturingWriter{ResponseWriter: w}
		next.ServeHTTP(captured, r)
		status := captured.status
		if status == 0 {
			status = http.StatusOK
		}
		method := r.Method
		if method != http.MethodGet && method != http.MethodPost {
			method = "OTHER"
		}
		govarHTTPRequests.WithLabelValues(endpoint, method, strconv.Itoa(status/100)+"xx").Inc()
		govarHTTPDuration.WithLabelValues(endpoint, method).Observe(time.Since(started).Seconds())
		finishHTTPServerSpan(span, status, started)
	})
}

func boundedEndpoint(path string) string {
	switch {
	case path == "/healthz":
		return "healthz"
	case path == "/readyz":
		return "readyz"
	case path == "/metrics":
		return "metrics"
	case path == "/v1/admit":
		return "admit"
	case path == "/v1/dispatch":
		return "dispatch"
	case path == "/v1/settle":
		return "settle"
	case path == "/v1/cancel":
		return "cancel"
	case strings.HasPrefix(path, "/v1/liability/"):
		return "liability"
	default:
		return "unknown"
	}
}

func recordAdmissionDecision(ctx context.Context, decision govar.Decision, reason govar.ReasonCode) {
	govarAdmissionDecisions.WithLabelValues(string(decision), string(reason)).Inc()
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("govar.decision", string(decision)), attribute.String("govar.reason_code", string(reason)))
}

func recordLedgerTransition(ctx context.Context, transition string, reason govar.ReasonCode, success bool) {
	govarLedgerTransitions.WithLabelValues(transition, string(reason), strconv.FormatBool(success)).Inc()
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("govar.transition", transition), attribute.String("govar.reason_code", string(reason)), attribute.Bool("govar.success", success))
}
