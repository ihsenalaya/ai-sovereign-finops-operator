package govarextproc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/imperium/ai-sovereign-finops-operator/internal/govar"
)

// Server is the synchronous Envoy ext_proc trust boundary. Envoy terminates
// downstream TLS and sends buffered request/response bodies on this stream.
type Server struct {
	extprocv3.UnimplementedExternalProcessorServer
	AdmissionURL       string
	MasterSecret       []byte
	HTTPClient         *http.Client
	PrincipalRegistry  map[string]RouteBinding
	ResolvePrincipal   func(context.Context, string) (RouteBinding, error)
	AllowedGatewayURIs map[string]struct{}
	AllowInsecureDev   bool
}

type RouteBinding struct {
	Namespace, TenantID, WorkloadUID, Team, Application string
	BudgetPolicy, RoutingPolicy                         string
	Sensitive                                           bool
	AllowedZones                                        []string
}

type RouteTarget struct {
	SelectedModel      string
	ProviderDeployment string
	Cluster            string
	Authority          string
	PathMode           string
	Path               string
	SnapshotHash       string
}

type streamState struct {
	headers                   map[string]string
	requestBody, responseBody []byte
	requestID, attemptID      string
	binding                   RouteBinding
	route                     RouteTarget
	routeSnapshotHash         string
}

const maxExtProcBodyBytes = 1 << 20

func (s *Server) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	gatewayURI, err := s.authorizeGatewayTransport(stream.Context())
	if err != nil {
		return err
	}
	state := streamState{headers: map[string]string{}}
	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch {
		case request.GetRequestHeaders() != nil:
			state.headers = headerMap(request.GetRequestHeaders().GetHeaders())
			binding, err := s.resolvePrincipal(stream.Context(), gatewayURI, state.headers["x-forwarded-client-cert"])
			if err != nil {
				return stream.Send(immediate(http.StatusForbidden, err.Error()))
			}
			state.binding = binding
			if err := stream.Send(headerContinue(true)); err != nil {
				return err
			}
		case request.GetRequestBody() != nil:
			body := request.GetRequestBody()
			if len(state.requestBody)+len(body.GetBody()) > maxExtProcBodyBytes {
				return stream.Send(immediate(http.StatusRequestEntityTooLarge, "governed request body exceeds 1 MiB"))
			}
			state.requestBody = append(state.requestBody, body.GetBody()...)
			if !body.GetEndOfStream() {
				if err := stream.Send(bodyContinue(true, RouteTarget{}, nil)); err != nil {
					return err
				}
				continue
			}
			_, err := s.admitAndClaim(stream.Context(), &state)
			if err != nil {
				return stream.Send(immediate(http.StatusForbidden, err.Error()))
			}
			if err := stream.Send(bodyContinue(true, state.route, state.requestBody)); err != nil {
				return err
			}
		case request.GetResponseHeaders() != nil:
			if state.requestID != "" {
				if err := s.dispatch(stream.Context(), state, "delivered", "DELIVERED"); err != nil {
					return err
				}
			}
			if err := stream.Send(responseHeaderContinue()); err != nil {
				return err
			}
		case request.GetResponseBody() != nil:
			body := request.GetResponseBody()
			if len(state.responseBody)+len(body.GetBody()) > maxExtProcBodyBytes {
				return errors.New("provider response body exceeds 1 MiB; liability remains unresolved")
			}
			state.responseBody = append(state.responseBody, body.GetBody()...)
			if body.GetEndOfStream() && state.requestID != "" {
				usage, ok := parseUsage(state.responseBody)
				if ok {
					if err := s.settle(stream.Context(), state, usage); err != nil {
						return err
					}
				} else {
					if err := s.cancel(stream.Context(), state, "missing_or_invalid_usage"); err != nil {
						return err
					}
				}
			}
			if err := stream.Send(responseBodyContinue()); err != nil {
				return err
			}
		default:
			return errors.New("unsupported ext_proc message")
		}
	}
}

func (s *Server) authorizeGatewayTransport(ctx context.Context) (string, error) {
	if s.AllowInsecureDev {
		return "insecure-dev", nil
	}
	remote, ok := peer.FromContext(ctx)
	if !ok {
		return "", errors.New("authenticated Envoy mTLS transport is required")
	}
	tlsInfo, ok := remote.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.PeerCertificates) == 0 || len(tlsInfo.State.VerifiedChains) == 0 {
		return "", errors.New("verified Envoy client certificate is required")
	}
	for _, uri := range tlsInfo.State.PeerCertificates[0].URIs {
		if _, allowed := s.AllowedGatewayURIs[uri.String()]; allowed {
			return uri.String(), nil
		}
	}
	return "", errors.New("Envoy client certificate SPIFFE identity is not allowed")
}

type admitResult struct {
	Decision           string               `json:"decision"`
	ReasonCode         string               `json:"reason_code"`
	SelectedDeployment string               `json:"selected_deployment"`
	ProviderAttemptID  string               `json:"provider_attempt_id"`
	ApprovalRef        string               `json:"approval_ref,omitempty"`
	PricingVersion     string               `json:"pricing_version"`
	RouteSnapshot      *govar.RouteSnapshot `json:"route_snapshot"`
}

func (s *Server) admitAndClaim(ctx context.Context, state *streamState) (admitResult, error) {
	var payload map[string]any
	if err := json.Unmarshal(state.requestBody, &payload); err != nil {
		return admitResult{}, errors.New("malformed LLM request body")
	}
	if streaming, _ := payload["stream"].(bool); streaming {
		return admitResult{}, errors.New("stream=true is unsupported until authoritative streaming settlement is implemented")
	}
	maxOutput := int64Value(payload["max_tokens"])
	if maxOutput == 0 {
		maxOutput = int64Value(payload["max_output_tokens"])
	}
	state.requestID = first(state.headers["x-request-id"], state.headers["x-govar-request-id"])
	if state.requestID == "" {
		return admitResult{}, errors.New("trusted Envoy route must provide x-request-id")
	}
	binding := state.binding
	request := map[string]any{"request_id": state.requestID, "namespace": binding.Namespace, "tenant_id": binding.TenantID,
		"workload_uid": binding.WorkloadUID, "team": binding.Team, "application": binding.Application,
		"budget_policy_name": binding.BudgetPolicy, "routing_policy_name": binding.RoutingPolicy,
		"sensitive_data": binding.Sensitive, "allowed_zones": binding.AllowedZones,
		"input_tokens": int64(0), "input_tokens_exact": false, "max_output_tokens": maxOutput}
	if approvalRef := strings.TrimSpace(state.headers["x-govar-approval-ref"]); approvalRef != "" {
		request["approval_ref"] = approvalRef
	}
	var result admitResult
	if err := s.post(ctx, *state, "/v1/admit", request, &result); err != nil {
		return result, err
	}
	if result.Decision != "ADMIT" || result.ReasonCode == "duplicate_request" {
		return result, fmt.Errorf("admission decision=%s reason=%s approval_ref=%s", result.Decision, result.ReasonCode, result.ApprovalRef)
	}
	state.attemptID = result.ProviderAttemptID
	if result.RouteSnapshot == nil || result.SelectedDeployment != result.RouteSnapshot.ModelName || result.PricingVersion != result.RouteSnapshot.PricingVersion {
		return result, errors.New("admission response route snapshot conflicts with top-level identity")
	}
	if err := govar.ValidateRouteSnapshot(*result.RouteSnapshot); err != nil {
		return result, err
	}
	route := RouteTarget{SelectedModel: result.RouteSnapshot.ModelName, ProviderDeployment: result.RouteSnapshot.ProviderDeployment,
		Cluster: result.RouteSnapshot.Cluster, Authority: result.RouteSnapshot.Authority, PathMode: result.RouteSnapshot.PathMode,
		SnapshotHash: result.RouteSnapshot.SnapshotHash}
	state.routeSnapshotHash = result.RouteSnapshot.SnapshotHash
	rewritten, err := rewriteRequestForRoute(state.requestBody, state.headers[":path"], route)
	if err != nil {
		return result, err
	}
	route.Path, err = routePath(state.headers[":path"], route)
	if err != nil {
		return result, err
	}
	state.route, state.requestBody = route, rewritten
	if err := s.dispatch(ctx, *state, "claim", "CLAIMED"); err != nil {
		return result, err
	}
	return result, nil
}

func rewriteRequestForRoute(body []byte, originalPath string, target RouteTarget) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("malformed LLM request body")
	}
	switch target.PathMode {
	case "openai-body", "anthropic-body":
		payload["model"] = target.ProviderDeployment
	case "azure-deployment-path":
		delete(payload, "model")
	case "google-generate-path":
		delete(payload, "model")
	default:
		return nil, fmt.Errorf("unsupported route path mode %q", target.PathMode)
	}
	return json.Marshal(payload)
}

func routePath(original string, target RouteTarget) (string, error) {
	path, query, _ := strings.Cut(first(original, "/v1/chat/completions"), "?")
	lowerPath := strings.ToLower(strings.TrimSuffix(path, "/"))
	switch target.PathMode {
	case "openai-body":
		if !isOpenAIPath(lowerPath) {
			return "", fmt.Errorf("OpenAI body adapter does not support request path %q", path)
		}
		// Preserve the API shape; only the provider model in the body changes.
	case "anthropic-body":
		if lowerPath != "/v1/messages" {
			return "", fmt.Errorf("Anthropic body adapter does not support request path %q", path)
		}
	case "azure-deployment-path":
		suffix := ""
		for _, candidate := range []string{"/chat/completions", "/responses", "/completions", "/embeddings"} {
			if strings.HasSuffix(lowerPath, candidate) {
				suffix = candidate
				break
			}
		}
		if suffix == "" {
			return "", fmt.Errorf("Azure deployment adapter does not support request path %q", path)
		}
		path = "/openai/deployments/" + url.PathEscape(target.ProviderDeployment) + suffix
	case "google-generate-path":
		if !strings.HasSuffix(lowerPath, ":generatecontent") {
			return "", fmt.Errorf("Google generate adapter does not support request path %q", path)
		}
		path = "/v1beta/models/" + url.PathEscape(target.ProviderDeployment) + ":generateContent"
	default:
		return "", fmt.Errorf("unsupported route path mode %q", target.PathMode)
	}
	if query != "" {
		path += "?" + query
	}
	return path, nil
}

func isOpenAIPath(path string) bool {
	switch path {
	case "/v1/chat/completions", "/v1/responses", "/v1/completions", "/v1/embeddings", "/v1/rerank":
		return true
	}
	return false
}

func (s *Server) dispatch(ctx context.Context, state streamState, suffix, status string) error {
	return s.post(ctx, state, "/v1/dispatch", map[string]any{"request_id": state.requestID, "event_id": state.requestID + ":extproc:" + suffix,
		"tenant_id": state.binding.TenantID, "workload_uid": state.binding.WorkloadUID, "provider_attempt_id": state.attemptID, "route_snapshot_hash": state.routeSnapshotHash, "status": status}, nil)
}
func (s *Server) settle(ctx context.Context, state streamState, usage usageSummary) error {
	return s.post(ctx, state, "/v1/settle", map[string]any{"request_id": state.requestID, "settlement_id": state.requestID + ":extproc:settle",
		"tenant_id": state.binding.TenantID, "workload_uid": state.binding.WorkloadUID, "provider_attempt_id": state.attemptID, "actual_cost_micros": 0,
		"actual_input_tokens": usage.Input, "actual_output_tokens": usage.Output, "usage_version": 1, "final": true}, nil)
}
func (s *Server) cancel(ctx context.Context, state streamState, reason string) error {
	return s.post(ctx, state, "/v1/cancel", map[string]any{"request_id": state.requestID, "event_id": state.requestID + ":extproc:cancel:" + reason,
		"tenant_id": state.binding.TenantID, "workload_uid": state.binding.WorkloadUID, "provider_attempt_id": state.attemptID, "reason": reason}, nil)
}

func (s *Server) post(ctx context.Context, state streamState, path string, payload any, out any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.AdmissionURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	namespace, tenant, workload := state.binding.Namespace, state.binding.TenantID, state.binding.WorkloadUID
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	digest := sha256.Sum256(body)
	message := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n%x", timestamp, req.Method, req.URL.EscapedPath(), tenant, workload, namespace, digest)
	key := deriveKey(s.MasterSecret, namespace, tenant, workload)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GOVAR-Tenant-ID", tenant)
	req.Header.Set("X-GOVAR-Workload-UID", workload)
	req.Header.Set("X-GOVAR-Namespace", namespace)
	req.Header.Set("X-GOVAR-Timestamp", timestamp)
	req.Header.Set("X-GOVAR-Signature", fmt.Sprintf("%x", mac.Sum(nil)))
	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("%s returned %d: %s", path, response.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		return json.NewDecoder(response.Body).Decode(out)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

func deriveKey(master []byte, namespace, tenant, workload string) []byte {
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte("govar-identity-v2\x00" + namespace + "\x00" + tenant + "\x00" + workload))
	return []byte(fmt.Sprintf("%x", mac.Sum(nil)))
}
func (s *Server) resolvePrincipal(ctx context.Context, gatewayURI, xfcc string) (RouteBinding, error) {
	fields := map[string]string{}
	for _, part := range strings.FieldsFunc(xfcc, func(r rune) bool { return r == ';' || r == ',' }) {
		part = strings.TrimSpace(part)
		key, value, ok := strings.Cut(part, "=")
		if !ok || fields[key] != "" {
			return RouteBinding{}, errors.New("XFCC is not a single SANITIZE_SET client certificate record")
		}
		fields[key] = strings.Trim(value, `"`)
	}
	uri := fields["URI"]
	if uri == "" {
		return RouteBinding{}, errors.New("sanitized downstream mTLS URI identity is required")
	}
	if gatewayURI != "insecure-dev" && fields["By"] != gatewayURI {
		return RouteBinding{}, errors.New("XFCC By identity does not match the authenticated Envoy certificate")
	}
	hash, hashErr := hex.DecodeString(fields["Hash"])
	if hashErr != nil || len(hash) != sha256.Size {
		return RouteBinding{}, errors.New("XFCC client certificate hash is not a SHA-256 SANITIZE_SET value")
	}
	if s.ResolvePrincipal != nil {
		return s.ResolvePrincipal(ctx, uri)
	}
	binding, ok := s.PrincipalRegistry[uri]
	if !ok || binding.Namespace == "" || binding.TenantID == "" || binding.WorkloadUID == "" || binding.BudgetPolicy == "" || binding.RoutingPolicy == "" {
		return RouteBinding{}, errors.New("mTLS principal is not bound to a complete server-side workload registry entry")
	}
	return binding, nil
}
func headerMap(m *corev3.HeaderMap) map[string]string {
	out := map[string]string{}
	if m == nil {
		return out
	}
	for _, h := range m.Headers {
		out[strings.ToLower(h.Key)] = first(h.Value, string(h.RawValue))
	}
	return out
}
func headerContinue(clear bool) *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_RequestHeaders{RequestHeaders: &extprocv3.HeadersResponse{Response: &extprocv3.CommonResponse{Status: extprocv3.CommonResponse_CONTINUE, ClearRouteCache: clear}}}}
}
func bodyContinue(clear bool, route RouteTarget, body []byte) *extprocv3.ProcessingResponse {
	common := &extprocv3.CommonResponse{Status: extprocv3.CommonResponse_CONTINUE, ClearRouteCache: clear}
	if route.SelectedModel != "" {
		set := func(key, value string) *corev3.HeaderValueOption {
			return &corev3.HeaderValueOption{Header: &corev3.HeaderValue{Key: key, RawValue: []byte(value)}, AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD}
		}
		common.HeaderMutation = &extprocv3.HeaderMutation{SetHeaders: []*corev3.HeaderValueOption{
			set("x-ai-eg-model", route.SelectedModel), set("x-govar-upstream-cluster", route.Cluster), set("x-govar-upstream-authority", route.Authority), set(":path", route.Path),
			set("x-govar-route-snapshot-hash", route.SnapshotHash),
			set("content-length", strconv.Itoa(len(body))),
		}}
		common.BodyMutation = &extprocv3.BodyMutation{Mutation: &extprocv3.BodyMutation_Body{Body: body}}
	}
	return &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_RequestBody{RequestBody: &extprocv3.BodyResponse{Response: common}}}
}
func responseHeaderContinue() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_ResponseHeaders{ResponseHeaders: &extprocv3.HeadersResponse{Response: &extprocv3.CommonResponse{Status: extprocv3.CommonResponse_CONTINUE}}}}
}
func responseBodyContinue() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_ResponseBody{ResponseBody: &extprocv3.BodyResponse{Response: &extprocv3.CommonResponse{Status: extprocv3.CommonResponse_CONTINUE}}}}
}
func immediate(status int, body string) *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{Response: &extprocv3.ProcessingResponse_ImmediateResponse{ImmediateResponse: &extprocv3.ImmediateResponse{Status: &typev3.HttpStatus{Code: typev3.StatusCode(status)}, Body: body, Details: "govar_ext_proc_denied"}}}
}

type usageSummary struct{ Input, Output int64 }

func parseUsage(body []byte) (usageSummary, bool) {
	var raw struct {
		Usage *map[string]json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(body, &raw) != nil || raw.Usage == nil {
		return usageSummary{}, false
	}
	read := func(keys ...string) (int64, bool) {
		for _, k := range keys {
			if v, ok := (*raw.Usage)[k]; ok {
				var n int64
				if json.Unmarshal(v, &n) == nil && n >= 0 {
					return n, true
				}
				return 0, false
			}
		}
		return 0, false
	}
	in, ok1 := read("prompt_tokens", "input_tokens")
	out, ok2 := read("completion_tokens", "output_tokens")
	return usageSummary{in, out}, ok1 && ok2
}
func int64Value(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case json.Number:
		x, _ := n.Int64()
		return x
	}
	return 0
}
func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
