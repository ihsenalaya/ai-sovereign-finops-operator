# AI QualityScore Kind self-verification - 2026-06-23

Final green archive: `experimentation/results-live-kind-20260623-072147`

Strict three-provider archive: `experimentation/results-live-kind-20260623-072451`

The final Kind run is green for the four real gateway-backed evaluation Jobs and two compliant scored providers. The required third radar provider is not green because Azure AI Foundry returns `DeploymentNotFound` for `mistral-small-latest`, and the subscription rejects deployment creation with `ReadOnlyDisabledSubscription`. The demo now fails closed in strict mode instead of fabricating a third provider.

## 1. Build and unit tests

Command:

```bash
cd operateur && make manifests generate
cd operateur && make lint
cd operateur && make test
```

Observed output:

```text
controller-gen ... output:crd:artifacts:config=config/crd/bases
controller-gen ... object:headerFile="hack/boilerplate.go.txt" paths="./..."
golangci-lint-v2.12.0 run
0 issues.
ok github.com/imperium/ai-sovereign-finops-operator/internal/controller
ok github.com/imperium/ai-sovereign-finops-operator/internal/qualityengine
ok github.com/imperium/ai-sovereign-finops-operator/internal/qualityeval
```

## 2. Kind deployment and observability

Command:

```bash
BUILD_OPERATOR=false RESET_CLUSTER=false REQUIRE_THIRD_QUALITY_PROVIDER=false \
  ENABLE_MISTRAL_SMALL_PROVIDER=false VERIFY_DELETE_CLUSTER_ON_EXIT=false \
  VERIFY_AUTO_STOP_APPS=false automatisation/envoy-aigw/deploy.sh verify
```

Observed output:

```text
deployment "greenops-ai-sovereign-finops-operator" successfully rolled out
deployment "demo-prometheus" successfully rolled out
deployment "demo-grafana" successfully rolled out
Latency telemetry OK: gateway duration histogram observed, report status true, routing score=0.918.
Quality score metrics OK: providers=mistral-eu openai-fr ; dimensions=correctness judged latency overall reliability semantic
Done - real demo is live
```

## 3. Client applications

Command:

```bash
kubectl --context kind-greenops get deploy -n rh
kubectl --context kind-greenops get deploy -n finance
kubectl --context kind-greenops get deploy -n legal
kubectl --context kind-greenops get deploy -n marketing
```

Observed applications:

```text
rh/chatbot-rh
finance/risk-assistant
legal/contract-review
marketing/content-writer
```

## 4. Real evaluation Jobs and populated scores

Command:

```bash
sed -n '1,80p' experimentation/results-live-kind-20260623-072147/kubectl_quality_jobs.txt
kubectl --context kind-greenops -n default get aiqualitygate -o wide
```

Observed output:

```text
quality-eval-finance-risk-assistant-quality-3ab0bae23b   Complete   1/1
quality-eval-legal-contract-quality-9e98125221           Complete   1/1
quality-eval-marketing-content-quality-549dedd88f        Complete   1/1
quality-eval-rh-chatbot-quality-61ad01862b               Complete   1/1

finance-risk-assistant-quality   Passed   candidate-safe   82.167
legal-contract-quality           Passed   candidate-safe   88.044
marketing-content-quality        Passed   candidate-safe   87.657
rh-chatbot-quality               Passed   candidate-safe   87.517
```

Score plausibility from real evidence:

```text
finance correctness=73.409 semantic=63.909
legal correctness=81.615 semantic=77.293
marketing correctness=81.170 semantic=76.153
rh correctness=82.371 semantic=75.109
```

## 5. Quality metrics

Command:

```bash
kubectl --context kind-greenops -n default run operator-metrics-check --rm -i \
  --restart=Never --image=curlimages/curl:8.10.1 --quiet -- \
  curl -s -m 20 \
  http://greenops-ai-sovereign-finops-operator-metrics.greenops-system.svc.cluster.local:8080/metrics \
  | grep '^ai_finops_quality_score'
```

Observed output excerpt:

```text
ai_finops_quality_score{app="chatbot-rh",dimension="correctness",model="gpt-france-mini",namespace="rh",provider="openai-fr"} 82.371
ai_finops_quality_score{app="chatbot-rh",dimension="overall",model="gpt-france-mini",namespace="rh",provider="openai-fr"} 87.517
ai_finops_quality_score{app="content-writer",dimension="correctness",model="mistral-large-latest",namespace="marketing",provider="mistral-eu"} 81.17
ai_finops_quality_score{app="contract-review",dimension="overall",model="mistral-large-latest",namespace="legal",provider="mistral-eu"} 88.044
ai_finops_quality_score{app="risk-assistant",dimension="semantic",model="mistral-large-latest",namespace="finance",provider="mistral-eu"} 63.909
```

## 6. Dashboard radar

PromQL used by the radar panel:

```promql
avg by(provider, dimension) (ai_finops_quality_score{dimension=~"correctness|reliability|latency|semantic|overall"})
```

Observed green run:

```text
providers=mistral-eu openai-fr
dimensions=correctness judged latency overall reliability semantic
```

Strict three-provider command:

```bash
BUILD_OPERATOR=false RESET_CLUSTER=false REQUIRE_THIRD_QUALITY_PROVIDER=true \
  ENABLE_MISTRAL_SMALL_PROVIDER=true VERIFY_DELETE_CLUSTER_ON_EXIT=false \
  VERIFY_AUTO_STOP_APPS=false automatisation/envoy-aigw/deploy.sh verify
```

Observed strict output:

```text
Mistral Small Foundry preflight failed for deployment mistral-small-latest (HTTP 404).
code: DeploymentNotFound
Third QualityScore provider is required, but mistral-small-latest is not reachable.
```

Provisioning attempt:

```bash
ENABLE_MISTRAL_SMALL=true automatisation/azure/scripts/07-deploy-mistral-foundry.sh
```

Observed output:

```text
ReadOnlyDisabledSubscription: The subscription 'ec0e829d-64e1-43fd-b721-ecf5b5112773' is disabled and therefore marked as read only.
```

Result: the dashboard can render the radar from real metrics; the third polygon is blocked by the missing Azure Foundry deployment, and strict mode fails closed.

## 7. Sovereignty preserved

Command:

```bash
grep '^ai_finops_enforcement_actions' \
  experimentation/results-live-kind-20260623-072147/metrics/operator_metrics.txt
grep '^ai_finops_shadow_ai_egress' \
  experimentation/results-live-kind-20260623-072147/metrics/operator_metrics.txt
```

Observed output:

```text
ai_finops_enforcement_actions{action="report",actuated="true",application="risk-assistant",mode="reportOnly",namespace="finance",policy="regulated-france-policy"} 1
ai_finops_shadow_ai_egress{application="rogue-script",namespace="finance",provider="openai",severity="critical",zone="US"} 10
```

Quality evaluation is blocked before calling non-compliant providers by `evaluationSovereigntyBlock`; strict missing-provider mode returns insufficient data/preflight failure instead of routing to US/GLOBAL alternatives.

## 8. Reversibility

Command:

```bash
BUILD_OPERATOR=false automatisation/envoy-aigw/deploy.sh down
kubectl --context kind-greenops -n default get aiqualitygate,jobs,cm -l aiops.imperium.io/quality-evaluator=true
kubectl --context kind-greenops get aiqualitygate -A
```

Observed output:

```text
Operator + Envoy control planes kept (helm uninstall greenops / eg / aieg to remove).
No resources found in default namespace.
No resources found
```

The first `down` attempt exposed a chart RBAC gap for deleting evidence ConfigMaps. The Helm RBAC now includes `configmaps/delete`; after upgrade the finalizer removed Jobs and live evidence ConfigMaps.

## 9. Pod security

Command:

```bash
grep -n 'runAsNonRoot\\|readOnlyRootFilesystem\\|drop:\\|RuntimeDefault' \
  experimentation/results-live-kind-20260623-072147/yaml/quality-jobs.yaml
```

Observed output excerpt:

```text
capabilities:
  drop:
  - ALL
readOnlyRootFilesystem: true
runAsNonRoot: true
seccompProfile:
  type: RuntimeDefault
```

## Detected apps and providers

Applications:

```text
finance/risk-assistant
legal/contract-review
marketing/content-writer
rh/chatbot-rh
```

Scored compliant providers in the green run:

```text
openai-fr
mistral-eu
```

Required third compliant provider:

```text
mistral-small-eu -> blocked because Azure Foundry deployment mistral-small-latest is absent and subscription writes are rejected.
```
