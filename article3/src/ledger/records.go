package ledger

import "fmt"

type ReservationStatus string

const (
	StatusReserved ReservationStatus = "reserved"
	StatusSettled  ReservationStatus = "settled"
	StatusExpired  ReservationStatus = "expired"
	StatusFailed   ReservationStatus = "failed"
)

type ReservationRecord struct {
	RequestID        string
	TenantID         string
	Model            string
	ReservedAmount   float64
	SettledAmount    float64
	Status           ReservationStatus
	SettlementEventID string
}

type RequestLedger struct {
	records map[string]ReservationRecord
}

func NewRequestLedger() *RequestLedger {
	return &RequestLedger{
		records: make(map[string]ReservationRecord),
	}
}

func (l *RequestLedger) Reserve(state *TenantState, requestID string, model string, amount float64) error {
	if l == nil {
		return fmt.Errorf("ledger is nil")
	}
	if state == nil {
		return fmt.Errorf("tenant state is nil")
	}
	if requestID == "" {
		return fmt.Errorf("request id is required")
	}
	if _, exists := l.records[requestID]; exists {
		return fmt.Errorf("request %q already exists", requestID)
	}
	if err := state.Reserve(amount); err != nil {
		return err
	}
	l.records[requestID] = ReservationRecord{
		RequestID:      requestID,
		TenantID:       state.TenantID,
		Model:          model,
		ReservedAmount: amount,
		Status:         StatusReserved,
	}
	return nil
}

func (l *RequestLedger) Get(requestID string) (ReservationRecord, bool) {
	if l == nil {
		return ReservationRecord{}, false
	}
	rec, ok := l.records[requestID]
	return rec, ok
}

func (l *RequestLedger) SettleOnce(state *TenantState, requestID string, eventID string, actual float64) error {
	if l == nil {
		return fmt.Errorf("ledger is nil")
	}
	if state == nil {
		return fmt.Errorf("tenant state is nil")
	}
	rec, ok := l.records[requestID]
	if !ok {
		return fmt.Errorf("request %q not found", requestID)
	}
	if rec.TenantID != state.TenantID {
		return fmt.Errorf("tenant mismatch for request %q", requestID)
	}
	if rec.Status == StatusSettled {
		if rec.SettlementEventID == eventID {
			return nil
		}
		return fmt.Errorf("request %q already settled by another event", requestID)
	}
	if rec.Status != StatusReserved {
		return fmt.Errorf("request %q is not in reserved state", requestID)
	}
	if err := state.Settle(actual, rec.ReservedAmount); err != nil {
		return err
	}
	rec.Status = StatusSettled
	rec.SettledAmount = actual
	rec.SettlementEventID = eventID
	l.records[requestID] = rec
	return nil
}

func (l *RequestLedger) Expire(state *TenantState, requestID string) error {
	if l == nil {
		return fmt.Errorf("ledger is nil")
	}
	if state == nil {
		return fmt.Errorf("tenant state is nil")
	}
	rec, ok := l.records[requestID]
	if !ok {
		return fmt.Errorf("request %q not found", requestID)
	}
	if rec.TenantID != state.TenantID {
		return fmt.Errorf("tenant mismatch for request %q", requestID)
	}
	if rec.Status != StatusReserved {
		return fmt.Errorf("request %q is not in reserved state", requestID)
	}
	if err := state.Release(rec.ReservedAmount); err != nil {
		return err
	}
	rec.Status = StatusExpired
	l.records[requestID] = rec
	return nil
}
