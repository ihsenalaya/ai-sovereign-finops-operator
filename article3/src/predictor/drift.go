package predictor

type DriftReport struct {
	ReferenceMean float64
	RecentMean    float64
	Ratio         float64
	Drifted       bool
}

func DetectMeanRatioDrift(reference []int, recent []int, ratioThreshold float64) DriftReport {
	refMean := meanInts(reference)
	recentMean := meanInts(recent)
	ratio := 1.0
	if refMean > 0 {
		ratio = recentMean / refMean
	}
	drifted := false
	if refMean > 0 && ratioThreshold > 0 && ratio >= ratioThreshold {
		drifted = true
	}
	return DriftReport{
		ReferenceMean: refMean,
		RecentMean:    recentMean,
		Ratio:         ratio,
		Drifted:       drifted,
	}
}

func meanInts(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0
	for _, v := range values {
		total += v
	}
	return float64(total) / float64(len(values))
}
