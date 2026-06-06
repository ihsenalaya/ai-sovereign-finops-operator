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

// Package litellm prepares a TelemetryCollector backed by the LiteLLM admin/usage
// API. The MVP ships the interface and configuration surface; the live HTTP
// integration (auth, pagination, /spend/logs mapping) lands in a later sprint.
package litellm

import (
	"context"
	"errors"
	"time"

	"github.com/imperium/ai-sovereign-finops-operator/internal/collectors"
)

// ErrNotImplemented is returned until the live LiteLLM integration ships.
var ErrNotImplemented = errors.New("litellm collector: live integration not implemented in MVP; use fake or prometheus mode")

// Collector talks to a LiteLLM proxy admin API.
type Collector struct {
	Endpoint string // base URL, e.g. http://litellm.default.svc.cluster.local:4000
	APIKey   string // admin/virtual key (resolved from a Secret by the controller)
}

// New returns a LiteLLM collector for the given endpoint and key.
func New(endpoint, apiKey string) *Collector {
	return &Collector{Endpoint: endpoint, APIKey: apiKey}
}

// Name implements collectors.TelemetryCollector.
func (c *Collector) Name() string { return "litellm" }

// Collect implements collectors.TelemetryCollector.
//
// TODO(sprint): call LiteLLM /spend/logs (or /global/spend/report), map rows to
// UsageSample, handle pagination and the admin key auth header.
func (c *Collector) Collect(_ context.Context, _ time.Duration) ([]collectors.UsageSample, error) {
	return nil, ErrNotImplemented
}
