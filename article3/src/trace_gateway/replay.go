package tracegateway

import (
	"fmt"
	"sort"

	"github.com/imperium/ai-sovereign-finops-operator/article3/src/admission"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/ledger"
	"github.com/imperium/ai-sovereign-finops-operator/article3/src/predictor"
)

type CandidateSpec struct {
	Base                admission.Candidate
	PredictedOutputTokens []int
	InputCost           float64
	OutputCostPerToken  float64
}

type TraceRequest struct {
	RequestID       string
	TenantID        string
	Application     string
	ArrivalStep     int
	SettlementDelay int
	ActualCost      float64
	Candidates      []CandidateSpec
	MaxQueueRetries int
}

type EventType string

const (
	EventAdmitted EventType = "admitted"
	EventQueued   EventType = "queued"
	EventRejected EventType = "rejected"
	EventAbstained EventType = "abstained"
	EventSettled  EventType = "settled"
	EventExpired  EventType = "expired"
)

type Event struct {
	Step      int
	RequestID string
	TenantID  string
	Application string
	Type      EventType
	Model     string
	Amount    float64
	Reason    string
}

type ReplayConfig struct {
	Policy            admission.Policy
	QueueRetryDelay   int
	ReservationConfig predictor.ReservationConfig
}

type ReplayResult struct {
	Events  []Event
	Tenants map[string]ledger.TenantState
}

type pendingSettlement struct {
	step      int
	requestID string
	tenantID  string
	application string
	actual    float64
}

type queuedRequest struct {
	request TraceRequest
}

func Replay(requests []TraceRequest, tenantStates map[string]ledger.TenantState, cfg ReplayConfig) (ReplayResult, error) {
	tenants := make(map[string]ledger.TenantState, len(tenantStates))
	for k, v := range tenantStates {
		tenants[k] = v
	}
	reqs := append([]TraceRequest(nil), requests...)
	sort.SliceStable(reqs, func(i, j int) bool {
		if reqs[i].ArrivalStep == reqs[j].ArrivalStep {
			return reqs[i].RequestID < reqs[j].RequestID
		}
		return reqs[i].ArrivalStep < reqs[j].ArrivalStep
	})

	l := ledger.NewRequestLedger()
	var pendings []pendingSettlement
	var events []Event
	maxStep := 0
	for _, r := range reqs {
		if r.ArrivalStep > maxStep {
			maxStep = r.ArrivalStep
		}
		if r.ArrivalStep+r.SettlementDelay > maxStep {
			maxStep = r.ArrivalStep + r.SettlementDelay
		}
	}

	reqIdx := 0
	var queue []queuedRequest
	for step := 0; step <= maxStep; step++ {
		var arriving []TraceRequest
		for reqIdx < len(reqs) && reqs[reqIdx].ArrivalStep == step {
			arriving = append(arriving, reqs[reqIdx])
			reqIdx++
		}
		remainingQueue := queue[:0]
		for _, q := range queue {
			if q.request.ArrivalStep == step {
				arriving = append(arriving, q.request)
				continue
			}
			remainingQueue = append(remainingQueue, q)
		}
		queue = remainingQueue

		for _, r := range arriving {
			state, ok := tenants[r.TenantID]
			if !ok {
				return ReplayResult{}, fmt.Errorf("tenant %q not found", r.TenantID)
			}
			candidates := materializeCandidates(r.Candidates, cfg.ReservationConfig)
			decision := admission.Decide(cfg.Policy, state, candidates)
			switch decision.Action {
			case admission.ActionAdmit:
				if err := l.Reserve(&state, r.RequestID, decision.Model, decision.ReservationCost); err != nil {
					return ReplayResult{}, err
				}
				tenants[r.TenantID] = state
				events = append(events, Event{
					Step: step, RequestID: r.RequestID, TenantID: r.TenantID, Application: r.Application,
					Type: EventAdmitted, Model: decision.Model, Amount: decision.ReservationCost, Reason: decision.Reason,
				})
				pendings = append(pendings, pendingSettlement{
					step: step + r.SettlementDelay, requestID: r.RequestID, tenantID: r.TenantID, application: r.Application, actual: r.ActualCost,
				})
			case admission.ActionQueue:
				events = append(events, Event{
					Step: step, RequestID: r.RequestID, TenantID: r.TenantID, Application: r.Application,
					Type: EventQueued, Reason: decision.Reason,
				})
				if r.MaxQueueRetries > 0 {
					r.MaxQueueRetries--
					next := r
					delay := cfg.QueueRetryDelay
					if delay <= 0 {
						delay = 1
					}
					next.ArrivalStep = step + delay
					queue = append(queue, queuedRequest{request: next})
					if next.ArrivalStep > maxStep {
						maxStep = next.ArrivalStep
					}
				}
			case admission.ActionReject:
				events = append(events, Event{
					Step: step, RequestID: r.RequestID, TenantID: r.TenantID, Application: r.Application,
					Type: EventRejected, Reason: decision.Reason,
				})
			case admission.ActionAbstain:
				events = append(events, Event{
					Step: step, RequestID: r.RequestID, TenantID: r.TenantID, Application: r.Application,
					Type: EventAbstained, Reason: decision.Reason,
				})
			default:
				return ReplayResult{}, fmt.Errorf("unsupported decision action %q", decision.Action)
			}
		}

		for _, p := range pendings {
			if p.step != step {
				continue
			}
			state := tenants[p.tenantID]
			if err := l.SettleOnce(&state, p.requestID, "settlement:"+p.requestID, p.actual); err != nil {
				return ReplayResult{}, fmt.Errorf("settle request %s at step %d: %w", p.requestID, step, err)
			}
			tenants[p.tenantID] = state
			events = append(events, Event{
				Step: step, RequestID: p.requestID, TenantID: p.tenantID, Application: p.application,
				Type: EventSettled, Amount: p.actual,
			})
		}
	}

	return ReplayResult{
		Events:  events,
		Tenants: tenants,
	}, nil
}

func materializeCandidates(specs []CandidateSpec, cfg predictor.ReservationConfig) []admission.Candidate {
	out := make([]admission.Candidate, 0, len(specs))
	for _, spec := range specs {
		c := spec.Base
		if len(spec.PredictedOutputTokens) > 0 && spec.OutputCostPerToken > 0 {
			reservedTokens := predictor.ReservationBound(spec.PredictedOutputTokens, cfg)
			c.ReservationCost = spec.InputCost + float64(reservedTokens)*spec.OutputCostPerToken
		}
		out = append(out, c)
	}
	return out
}
