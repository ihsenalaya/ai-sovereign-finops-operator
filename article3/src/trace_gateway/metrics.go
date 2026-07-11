package tracegateway

type Metrics struct {
	AdmittedCount      int                `json:"admitted_count"`
	QueuedCount        int                `json:"queued_count"`
	RejectedCount      int                `json:"rejected_count"`
	AbstainedCount     int                `json:"abstained_count"`
	SettledCount       int                `json:"settled_count"`
	ReservedTotal      float64            `json:"reserved_total"`
	SettledTotal       float64            `json:"settled_total"`
	SlackTotal         float64            `json:"slack_total"`
	OvershootTotal     float64            `json:"overshoot_total"`
	TenantSettledTotal map[string]float64 `json:"tenant_settled_total"`
	ApplicationSettledTotal map[string]float64 `json:"application_settled_total"`
	ApplicationAdmittedCount map[string]int    `json:"application_admitted_count"`
}

func Summarize(result ReplayResult) Metrics {
	m := Metrics{
		TenantSettledTotal:      make(map[string]float64),
		ApplicationSettledTotal: make(map[string]float64),
		ApplicationAdmittedCount: make(map[string]int),
	}
	admittedByRequest := make(map[string]float64)
	for _, ev := range result.Events {
		switch ev.Type {
		case EventAdmitted:
			m.AdmittedCount++
			m.ReservedTotal += ev.Amount
			admittedByRequest[ev.RequestID] = ev.Amount
			m.ApplicationAdmittedCount[ev.Application]++
		case EventQueued:
			m.QueuedCount++
		case EventRejected:
			m.RejectedCount++
		case EventAbstained:
			m.AbstainedCount++
		case EventSettled:
			m.SettledCount++
			m.SettledTotal += ev.Amount
			m.TenantSettledTotal[ev.TenantID] += ev.Amount
			m.ApplicationSettledTotal[ev.Application] += ev.Amount
			if reserved, ok := admittedByRequest[ev.RequestID]; ok {
				if reserved > ev.Amount {
					m.SlackTotal += reserved - ev.Amount
				} else if ev.Amount > reserved {
					m.OvershootTotal += ev.Amount - reserved
				}
			}
		}
	}
	return m
}
