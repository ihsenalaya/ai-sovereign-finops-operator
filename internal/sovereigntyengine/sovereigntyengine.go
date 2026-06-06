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

// Package sovereigntyengine evaluates which providers are in use against a
// sovereignty policy (allowed/forbidden zones, sensitive-data rules) and emits
// findings. It is pure (no Kubernetes dependency) and never blocks traffic — the
// MVP is reportOnly and produces an audit-ready trail, not a legal guarantee.
package sovereigntyengine

import (
	"fmt"
	"sort"
	"strings"
)

// Severity levels for findings.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Finding is a single sovereignty observation.
type Finding struct {
	Severity string
	Message  string
	Provider string
	Zone     string
}

// ProviderInfo is the subset of an AIProvider needed to evaluate sovereignty.
type ProviderInfo struct {
	Name                    string
	Zone                    string // raw dataResidency (e.g. "france", "us")
	Managed                 bool
	AllowedForSensitiveData bool
}

// Policy is the subset of an AISovereigntyPolicy needed for evaluation.
type Policy struct {
	AllowedZones             []string
	ForbiddenZones           []string
	ExternalProvidersAllowed bool
	RequireAnonymization     bool
}

// euCountries are treated as covered when "EU" is an allowed zone.
var euCountries = map[string]bool{
	"FR": true, "DE": true, "ES": true, "IT": true, "NL": true, "BE": true,
	"LU": true, "IE": true, "PT": true, "AT": true, "FI": true, "SE": true,
	"DK": true, "PL": true, "CZ": true, "GR": true, "RO": true, "HU": true,
}

// NormalizeZone maps free-form residency strings to canonical zone codes.
func NormalizeZone(z string) string {
	switch strings.ToLower(strings.TrimSpace(z)) {
	case "fr", "france", "francecentral":
		return "FR"
	case "eu", "europe", "european union":
		return "EU"
	case "us", "usa", "united states", "eastus", "westus":
		return "US"
	case "cn", "china":
		return "CN"
	case "uk", "gb", "united kingdom":
		return "GB"
	case "":
		return ""
	default:
		return strings.ToUpper(strings.TrimSpace(z))
	}
}

func toSet(zones []string) map[string]bool {
	s := make(map[string]bool, len(zones))
	for _, z := range zones {
		s[NormalizeZone(z)] = true
	}
	return s
}

// zoneAllowed reports whether a normalized zone satisfies the allowed set. An EU
// member country is allowed when "EU" is permitted. An empty allowed set imposes
// no positive restriction (only the forbidden set applies).
func zoneAllowed(zone string, allowed map[string]bool) bool {
	if len(allowed) == 0 {
		return true
	}
	if allowed[zone] {
		return true
	}
	if allowed["EU"] && euCountries[zone] {
		return true
	}
	return false
}

// Evaluate returns the findings for the given providers under the policy.
// Findings are sorted by descending severity then provider for stable output.
func Evaluate(policy Policy, providers []ProviderInfo) []Finding {
	allowed := toSet(policy.AllowedZones)
	forbidden := toSet(policy.ForbiddenZones)

	var findings []Finding
	for _, p := range providers {
		zone := NormalizeZone(p.Zone)

		switch {
		case forbidden[zone]:
			findings = append(findings, Finding{
				Severity: SeverityCritical, Provider: p.Name, Zone: zone,
				Message: fmt.Sprintf("Provider %q operates in forbidden zone %s.", p.Name, zone),
			})
		case !zoneAllowed(zone, allowed):
			findings = append(findings, Finding{
				Severity: SeverityWarning, Provider: p.Name, Zone: zone,
				Message: fmt.Sprintf("Provider %q zone %s is outside the allowed zones %v.", p.Name, zone, sortedKeys(allowed)),
			})
		}

		if !policy.ExternalProvidersAllowed && p.Managed {
			findings = append(findings, Finding{
				Severity: SeverityWarning, Provider: p.Name, Zone: zone,
				Message: fmt.Sprintf("External managed provider %q is in use while external providers are disallowed for sensitive data.", p.Name),
			})
		}
	}

	if policy.RequireAnonymization {
		findings = append(findings, Finding{
			Severity: SeverityInfo,
			Message:  "Policy requires prompt anonymization; ensure the gateway redacts sensitive fields before egress.",
		})
	}

	sort.SliceStable(findings, func(i, j int) bool {
		ri, rj := severityRank(findings[i].Severity), severityRank(findings[j].Severity)
		if ri != rj {
			return ri > rj
		}
		return findings[i].Provider < findings[j].Provider
	})
	return findings
}

// CountBySeverity tallies findings per severity level.
func CountBySeverity(findings []Finding) map[string]int {
	out := map[string]int{SeverityInfo: 0, SeverityWarning: 0, SeverityCritical: 0}
	for _, f := range findings {
		out[f.Severity]++
	}
	return out
}

func severityRank(s string) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
