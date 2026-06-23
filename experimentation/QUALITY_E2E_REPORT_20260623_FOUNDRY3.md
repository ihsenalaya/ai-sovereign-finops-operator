# QualityScore Foundry 3-provider verification - 2026-06-23

Evidence directory: `experimentation/results-live-kind-20260623-081205-foundry3`

## Azure Foundry deployments

Command:

```bash
az account show --query '{name:name,id:id,state:state,tenantId:tenantId,isDefault:isDefault}' -o table
az cognitiveservices account deployment list -g greenops-rg -n greenops-foundry \
  --query '[].{name:name,model:properties.model.name,format:properties.model.format,version:properties.model.version,sku:sku.name,capacity:sku.capacity,state:properties.provisioningState}' \
  -o table
```

Observed output:

```text
Name    State    TenantId                              IsDefault
------  -------  ------------------------------------  -----------
FHIR    Enabled  f75c19a9-c006-47d7-98cc-f1b5638e6af6  True
Name                     Model             Format      Version     Sku               Capacity    State
-----------------------  ----------------  ----------  ----------  ----------------  ----------  ---------
mistral-large-latest     Mistral-Large-3   Mistral AI  1           DataZoneStandard  20          Succeeded
cohere-command-a-latest  cohere-command-a  Cohere      1           GlobalStandard    1           Succeeded
gpt-foundry-eu-mini      gpt-4.1-mini      OpenAI      2025-04-14  DataZoneStandard  10          Succeeded
```

## Strict Kind run

Command:

```bash
EVIDENCE_DIR="experimentation/results-live-kind-20260623-081205-foundry3" \
VERIFY_DELETE_CLUSTER_ON_EXIT=false BUILD_OPERATOR=true RESET_CLUSTER=false \
automatisation/envoy-aigw/deploy.sh verify
```

Observed key output:

```text
Mistral Foundry probe OK.
Third QualityScore Foundry provider probe OK (gpt-foundry-eu-mini).
job.batch/quality-eval-finance-risk-assistant-quality-3bc9c9a108 condition met
job.batch/quality-eval-legal-contract-quality-c75d3c60b7 condition met
job.batch/quality-eval-marketing-content-quality-e4c79b8b92 condition met
job.batch/quality-eval-rh-chatbot-quality-d6df45a7c2 condition met
Quality evaluation Jobs completed.
Latency telemetry OK: gateway duration histogram observed, report status true, routing score=0.841.
Quality score metrics OK: providers=mistral-eu openai-foundry-eu openai-fr ; dimensions=correctness judged latency overall reliability semantic
```

## Quality Jobs

Command:

```bash
kubectl --context kind-greenops -n default get jobs -l aiops.imperium.io/quality-evaluator=true \
  -o custom-columns=NAME:.metadata.name,STATUS:.status.conditions[-1].type,SUCCEEDED:.status.succeeded,FAILED:.status.failed --no-headers
```

Observed output:

```text
quality-eval-finance-risk-assistant-quality-3bc9c9a108   Complete   1     <none>
quality-eval-legal-contract-quality-c75d3c60b7           Complete   1     <none>
quality-eval-marketing-content-quality-e4c79b8b92        Complete   1     <none>
quality-eval-rh-chatbot-quality-d6df45a7c2               Complete   1     <none>
```

## AIQualityGate scores

Command:

```bash
kubectl --context kind-greenops -n default get aiqualitygate \
  -o custom-columns=NAME:.metadata.name,TARGET:.spec.target.application,SOURCE:.spec.sourceModel,CANDIDATE:.spec.candidateModel,PHASE:.status.phase,VERDICT:.status.verdict,SCORE:.status.qualityScore --no-headers
```

Observed output:

```text
finance-risk-assistant-quality   risk-assistant    gpt-france-mini        mistral-large-latest   Passed   candidate-safe   79.379
legal-contract-quality           contract-review   gpt-france-mini        mistral-large-latest   Passed   candidate-safe   85.347
marketing-content-quality        content-writer    gpt-france-mini        gpt-foundry-eu-mini    Failed   candidate-risk   85.162
rh-chatbot-quality               chatbot-rh        mistral-large-latest   gpt-france-mini        Passed   candidate-safe   83.636
```

`marketing-content-quality` is intentionally a real `candidate-risk` verdict: the Job succeeded and produced plausible data, while the candidate scored below the source plus tolerance.

## Radar PromQL

Dashboard PromQL:

```promql
avg by(provider, dimension) (ai_finops_quality_score{dimension=~"correctness|reliability|latency|semantic|overall"})
```

Observed Prometheus providers from that query: `openai-fr`, `openai-foundry-eu`, `mistral-eu`.

Observed representative result:

```text
openai-fr          correctness=81.718 latency=76.108 overall=83.636 reliability=100 semantic=74.457
openai-foundry-eu  correctness=80.762 latency=86.562 overall=85.162 reliability=100 semantic=75.714
mistral-eu         correctness=78.695 latency=89.167 overall=84.128 reliability=100 semantic=72.412
```

Full raw metrics are archived in `experimentation/results-live-kind-20260623-081205-foundry3/metrics/operator_metrics.txt`.

## Repository checks

Commands:

```bash
cd operateur
make manifests generate
make lint
make test
```

Observed output:

```text
make manifests generate: exit 0; generated API/CRD diff count = 0
make lint: 0 issues.
make test: exit 0; all non-e2e Go packages passed
```
