package predictor

type ReservationMode string

const (
	ModeQuantile ReservationMode = "quantile"
	ModeMeanStd  ReservationMode = "mean_std"
)

type ReservationConfig struct {
	Mode     ReservationMode
	Quantile float64
	ZScore   float64
}

func ReservationBound(samples []int, cfg ReservationConfig) int {
	switch cfg.Mode {
	case ModeMeanStd:
		return MeanStdBound(samples, cfg.ZScore)
	case ModeQuantile:
		fallthrough
	default:
		return QuantileBound(samples, cfg.Quantile)
	}
}
