package ledger

import (
	"fmt"
	"math"
)

const moneyEpsilon = 1e-9

type TenantState struct {
	TenantID string
	Budget   float64
	Settled  float64
	Reserved float64
}

func (t TenantState) Available() float64 {
	return normalizeAmount(t.Budget - t.Settled - t.Reserved)
}

func (t *TenantState) Reserve(amount float64) error {
	if amount < 0 {
		return fmt.Errorf("reservation must be non-negative")
	}
	if t.Available()+moneyEpsilon < amount {
		return fmt.Errorf("insufficient available budget")
	}
	t.Reserved = normalizeAmount(t.Reserved + amount)
	return nil
}

func (t *TenantState) Release(amount float64) error {
	if amount < 0 {
		return fmt.Errorf("release must be non-negative")
	}
	if amount > t.Reserved+moneyEpsilon {
		return fmt.Errorf("release exceeds reserved amount")
	}
	t.Reserved = normalizeAmount(t.Reserved - amount)
	return nil
}

func (t *TenantState) Settle(actual float64, reserved float64) error {
	if actual < 0 || reserved < 0 {
		return fmt.Errorf("settlement values must be non-negative")
	}
	if reserved > t.Reserved+moneyEpsilon {
		return fmt.Errorf("settlement exceeds reserved amount")
	}
	t.Reserved = normalizeAmount(t.Reserved - reserved)
	t.Settled = normalizeAmount(t.Settled + actual)
	return nil
}

func normalizeAmount(v float64) float64 {
	if math.Abs(v) < moneyEpsilon {
		return 0
	}
	return v
}
