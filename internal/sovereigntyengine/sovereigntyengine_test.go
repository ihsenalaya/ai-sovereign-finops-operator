/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sovereigntyengine

import "testing"

func TestEvaluate(t *testing.T) {
	policy := Policy{
		AllowedZones:             []string{"FR", "EU"},
		ForbiddenZones:           []string{"US", "CN"},
		ExternalProvidersAllowed: false,
		RequireAnonymization:     true,
	}
	providers := []ProviderInfo{
		{Name: "azure-openai-france", Zone: "france", Managed: true, AllowedForSensitiveData: true},
		{Name: "openai-us", Zone: "us", Managed: true, AllowedForSensitiveData: false},
		{Name: "onprem-gpu", Zone: "FR", Managed: false},
	}

	findings := Evaluate(policy, providers)
	counts := CountBySeverity(findings)

	// openai-us in US -> critical (forbidden). Both managed providers -> warning
	// (external disallowed). Anonymization -> info.
	if counts[SeverityCritical] != 1 {
		t.Errorf("critical = %d, want 1 (%+v)", counts[SeverityCritical], findings)
	}
	if counts[SeverityWarning] != 2 {
		t.Errorf("warning = %d, want 2 (%+v)", counts[SeverityWarning], findings)
	}
	if counts[SeverityInfo] != 1 {
		t.Errorf("info = %d, want 1", counts[SeverityInfo])
	}
	// Critical must sort first.
	if findings[0].Severity != SeverityCritical {
		t.Errorf("first finding severity = %s, want critical", findings[0].Severity)
	}
}

func TestZoneAllowedEUCoversFrance(t *testing.T) {
	allowed := toSet([]string{"EU"})
	if !zoneAllowed("FR", allowed) {
		t.Error("FR should be allowed when EU is permitted")
	}
	if zoneAllowed("US", allowed) {
		t.Error("US should not be allowed when only EU is permitted")
	}
}

func TestNormalizeZone(t *testing.T) {
	cases := map[string]string{"france": "FR", "francecentral": "FR", "us": "US", "eastus": "US", "": ""}
	for in, want := range cases {
		if got := NormalizeZone(in); got != want {
			t.Errorf("NormalizeZone(%q) = %q, want %q", in, got, want)
		}
	}
}
