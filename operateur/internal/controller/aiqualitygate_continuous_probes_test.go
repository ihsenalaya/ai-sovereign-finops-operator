package controller

import (
	"testing"
	"time"

	aiopsv1alpha1 "github.com/imperium/ai-sovereign-finops-operator/api/v1alpha1"
)

func samplePrompts(ids ...string) []goldenPrompt {
	out := make([]goldenPrompt, 0, len(ids))
	for _, id := range ids {
		out = append(out, goldenPrompt{ID: id, Prompt: "p-" + id})
	}
	return out
}

func TestSelectProbePromptsRoundRobinWalksAndWraps(t *testing.T) {
	prompts := samplePrompts("a", "b", "c", "d", "e")
	cp := &aiopsv1alpha1.AIQualityContinuousProbesSpec{SampleSize: 2, Strategy: "round-robin"}

	got := selectProbePrompts(prompts, cp, 0)
	if want := []string{"a", "b"}; !equalStrings(got, want) {
		t.Fatalf("cursor 0 = %v, want %v", got, want)
	}
	got = selectProbePrompts(prompts, cp, 2)
	if want := []string{"c", "d"}; !equalStrings(got, want) {
		t.Fatalf("cursor 2 = %v, want %v", got, want)
	}
	// Wrap around the end of the dataset.
	got = selectProbePrompts(prompts, cp, 4)
	if want := []string{"e", "a"}; !equalStrings(got, want) {
		t.Fatalf("cursor 4 (wrap) = %v, want %v", got, want)
	}
}

func TestSelectProbePromptsDefaultsToWholeDataset(t *testing.T) {
	prompts := samplePrompts("a", "b", "c")
	cp := &aiopsv1alpha1.AIQualityContinuousProbesSpec{} // no sampleSize
	got := selectProbePrompts(prompts, cp, 0)
	if len(got) != 3 {
		t.Fatalf("default sampleSize should replay all prompts, got %v", got)
	}
}

func TestSelectProbePromptsRandomIsSubsetWithoutRepeats(t *testing.T) {
	prompts := samplePrompts("a", "b", "c", "d", "e")
	cp := &aiopsv1alpha1.AIQualityContinuousProbesSpec{SampleSize: 3, Strategy: "random"}
	got := selectProbePrompts(prompts, cp, 1)
	if len(got) != 3 {
		t.Fatalf("random should return sampleSize ids, got %v", got)
	}
	seen := map[string]bool{}
	valid := map[string]bool{"a": true, "b": true, "c": true, "d": true, "e": true}
	for _, id := range got {
		if seen[id] {
			t.Fatalf("random returned a repeat: %v", got)
		}
		if !valid[id] {
			t.Fatalf("random returned unknown id %q", id)
		}
		seen[id] = true
	}
}

func TestProbeSampleSizeClamping(t *testing.T) {
	if probeSampleSize(0, 5) != 5 {
		t.Fatal("zero sampleSize should mean all prompts")
	}
	if probeSampleSize(10, 5) != 5 {
		t.Fatal("sampleSize larger than dataset should clamp to dataset size")
	}
	if probeSampleSize(3, 5) != 3 {
		t.Fatal("valid sampleSize should be kept")
	}
}

func TestParseProbeDurationDefaults(t *testing.T) {
	if d := parseProbeDuration("", time.Hour); d != time.Hour {
		t.Fatalf("empty should default, got %v", d)
	}
	if d := parseProbeDuration("garbage", time.Hour); d != time.Hour {
		t.Fatalf("invalid should default, got %v", d)
	}
	if d := parseProbeDuration("30m", time.Hour); d != 30*time.Minute {
		t.Fatalf("valid should parse, got %v", d)
	}
}

func TestDefaultedRetainRuns(t *testing.T) {
	if defaultedRetainRuns(0) != defaultProbeRetainRuns {
		t.Fatal("zero retainRuns should default")
	}
	if defaultedRetainRuns(7) != 7 {
		t.Fatal("explicit retainRuns should be kept")
	}
}

func TestProbeSamplesForModelKeepsEntryPerEvidence(t *testing.T) {
	prompts := samplePrompts("a", "b")
	// Two runs of the same prompt for the candidate model => two samples.
	score := 90.0
	evidence := []qualityEvidenceSample{
		{ID: "a", Model: "cand", Actual: "x1", SemanticScore: &score},
		{ID: "a", Model: "cand", Actual: "x2", SemanticScore: &score},
		{ID: "a", Model: "src", Actual: "y1"},
	}
	got := probeSamplesForModel(prompts, evidence, "cand")
	if len(got) != 2 {
		t.Fatalf("expected 2 candidate samples (one per evidence entry), got %d", len(got))
	}
	src := probeSamplesForModel(prompts, evidence, "src")
	if len(src) != 1 {
		t.Fatalf("expected 1 source sample, got %d", len(src))
	}
}

func TestParseProbeEvidenceWrappedAndBare(t *testing.T) {
	wrapped := `{"samples":[{"id":"a","model":"cand","actual":"hi"}]}`
	got, err := parseProbeEvidence(wrapped)
	if err != nil || len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("wrapped parse failed: %v %v", got, err)
	}
	bare := "- id: b\n  model: src\n  actual: yo\n"
	got, err = parseProbeEvidence(bare)
	if err != nil || len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("bare parse failed: %v %v", got, err)
	}
}

func TestTruncatedNameStaysWithinDNSLimit(t *testing.T) {
	long := "this-is-a-really-long-quality-gate-name-that-exceeds-limits-by-far"
	name := truncatedName("quality-probe-", long, "20260624-120000")
	if len(name) > 63 {
		t.Fatalf("name %q exceeds 63 chars (%d)", name, len(name))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
