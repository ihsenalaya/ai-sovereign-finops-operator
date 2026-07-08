# Scheduler Self-Security Report (Gate G8)

The attestation-aware scheduler is security-critical (it decides placement and
mints the placement token). This report documents that its own RBAC is minimal
and fail-closed, and how its signing key is protected. It is backed by live
`kubectl auth can-i` probes.

## Deliverables
- Harness: `article1/experiments/scheduler-security/run_scheduler_security_tests.sh`
- Main raw CSV: `article1/results/raw/aks/scheduler_security_tests.csv`
  (columns: timestamp,env,test_id,service_account,verb,resource,namespace,expected,actual,pass,raw_log_path)
- Manuscript: `article1/paper/sections/scheduler-security.tex`

## Minimal RBAC (from the chart, verified)
The scheduler ServiceAccount (`attestation-scheduler`) ClusterRole grants ONLY:
- read: pods, nodes, namespaces, confidentialinferencepolicies, attestationevidences
- create/update: aiplacementdecisions (+ /status)
- create: pods/binding, bindings, events
- coordination: leases
No wildcard verbs/resources; no cluster-admin; no blanket secrets read.

## Capability tests
Main-paper evidence currently uses the AKS S1-S12 run. The expanded S1-S30
script has passed on kind, but that is CI/regression only until rerun on AKS.

### AKS core (S1-S12)
FORBIDDEN (expected `no`): S1 create AttestationEvidence · S2 update
AttestationEvidence · S3 create RawAttestationReport · S4 update
ConfidentialInferencePolicy · S5 patch Nodes · S6 get Secrets · S10 delete
AttestationEvidence · S11 create MutatingWebhookConfiguration · S12 wildcard.

ALLOWED (expected `yes`): S7 get AttestationEvidence · S8 create
AIPlacementDecision · S9 create pods/binding.

## Run instructions
```bash
# AKS main evidence (auto-invoked by run_full_campaign.sh)
ENV_NAME=aks-real-sevsnp PLATFORM_NS=ai-platform \
  OUT_DIR=article1/results/raw/aks \
  bash article1/experiments/scheduler-security/run_scheduler_security_tests.sh

# kind regression only, not paper evidence
ENV_NAME=kind-live-simulated PLATFORM_NS=ai-platform \
  OUT_DIR=article1/results/raw/kind \
  CSV=article1/results/raw/kind/scheduler_security_tests_kind.csv \
  bash article1/experiments/scheduler-security/run_scheduler_security_tests.sh
```

## Signing-key protection
The Ed25519 placement-token private key lives in the Secret
`attestation-scheduler-signing-key`, mounted only by the scheduler. The
scheduler RBAC does NOT grant blanket secret read (S6), so a rogue scheduler
cannot obtain the key and cannot mint tokens that `verify-placement` accepts.
This composes with the trust chain: a compromised node cannot forge evidence
(A10), and a fake scheduler cannot forge a verifiable placement.

## Status
G8 main evidence = PASS_AKS_RBAC_CORE for S1-S12:
`article1/results/raw/aks/scheduler_security_tests.csv`.

Expanded S1-S30 = PASS_REGRESSION on kind:
`article1/results/raw/kind/scheduler_security_tests_kind.csv`.
Do not cite the kind S1-S30 result as main-paper evidence until it is rerun on
AKS.
