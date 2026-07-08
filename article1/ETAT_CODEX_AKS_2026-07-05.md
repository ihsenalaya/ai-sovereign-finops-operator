# Etat Codex AKS - 2026-07-05

Derniere mise a jour: 2026-07-05T19:11:06Z

Note 2026-07-06: ce fichier est historique. Il decrit l'arret volontaire et les
resultats partiels du 2026-07-05. Il est supersede par les resultats finaux AKS
GHCR `0.5.11` documentes dans `article1/suite.txt`,
`article1/status/q1_execution_report.md`, et le fichier d'etat final
`article1/ETAT_FINAL_AKS_2026-07-06.md`.

## Decision immediate

- L'utilisateur a demande d'arreter l'execution, d'ecrire ce fichier d'etat, puis de supprimer AKS.
- La campagne en cours a ete interrompue volontairement apres `run 7/30`.
- Les resultats partiels de la campagne interrompue ne doivent pas etre presentes comme resultats finaux du papier.

## AKS avant suppression

- Resource group: `rg-article1-confidential`
- Cluster: `aks-article1`
- Region: `westus2`
- Kubernetes: `1.35`
- Etat Azure avant suppression: `Succeeded`
- Le resource group ne contenait que la ressource AKS `aks-article1`.

## Plateforme deployee avant suppression

- Helm release: `ai-platform`
- Namespace: `ai-platform`
- Version images: `0.5.11`
- Registry: `ghcr.io/ihsenalaya/ai-sovereign-finops-operator`
- Aucun ACR ne doit etre reintroduit.

Images GHCR `0.5.11` verifiees:

- `controller`: `sha256:9f2a87731e5a5646595de6f3f28dca36d1c7546b9769bdb3697d41cb1890c463`
- `central-verifier`: `sha256:2f48bc01d6c242827f739a910eff68220a2e5f469ee37807be5c0bf1dcee2602`
- `node-attestation-agent`: `sha256:18ef99259d32c8b1baa9731f694d1fd3e9b293c5df20ff7232d1889601fba130`
- `attestation-scheduler`: `sha256:30c16a9a88167ff551e109281a75362bc7739c57ce4c116c0f23e28226b30f47`
- `key-release-gateway`: `sha256:30d0500f1c3aca30487c69c4b718eb2ac7a92d0043ec1dd67386e2380f70a857`
- `platform-api`: `sha256:d2010b6d36504c8d40a164375bbd239f580c8ef09113a573b2de5efd3270658b`
- `platform-ui`: `sha256:8dd0c84dd41ee3bc75732b61dfa43c895340ed01c9d2a2fea5cfef8df3e7c284`
- `thesis-bench`: `sha256:07adb6e3391f0bd2d11da804950ab1aed022c0b54033806b01bc23ead8682d53`

## Resultats verifies avant arret

- Attestation AKS reelle: `current_nodes=4 current_evidence=4 real=4 simulated=0 unverified=0 other=0`.
- Scheduler self-security G8: `30/30 PASS`.
- E2E positif AKS reel: `PASS`, pod `risk-assistant` bind sur un noeud `Standard_DC8as_v6`, verification independante `verify-placement PASS`.
- Campagne A1-A10 ancienne corrigee mais interrompue: apres relance avec namespaces isoles, le CSV courant contient `42` lignes, `42/42` bloquees, dernier run complet `run-7`.

Synthese partielle du CSV courant `article1/results/raw/aks/security_attacks_A1_A10.csv`:

- A1: `7/7`, observe `PENDING`, bloque par absence de bind sans evidence valide.
- A4: `7/7`, observe `PENDING`, bloque par incompatibilite TEE/policy.
- A5: `7/7`, observe `DENIED`, bloque par webhook runtimeClass interdit.
- A5b: `7/7`, observe `DENIED`, bloque par webhook annotation `model-digest` manquante.
- A8: `7/7`, observe `FAIL`, token falsifie rejete par `verify-placement`.
- A10: `7/7`, observe `see-csv`, separation RBAC/verifier/scheduler.

Important: ce `42/42` est un checkpoint partiel, pas un resultat final Q1. Il faut relancer `N_RUNS_SECURITY=30` sur un nouveau AKS avant de publier.

## Corrections techniques faites pendant cette reprise

- `0.5.10` a ete supersede par `0.5.11` apres decouverte d'un bug webhook: annotation `policy-hash` ajoutee en memoire mais patch non renvoye si le pod avait deja `schedulerName`, `runtimeClassName` et evidence.
- Fix webhook: detection de changement d'annotations autour de `annotateConfidentialPod`.
- Test ajoute: `TestConfidentialAnnotationsPatchWhenSchedulingAlreadySet`.
- Scheduler durci: decision `pending` avant PreBind, token seulement apres PreBind reussi.
- Harness securite durci:
  - namespaces uniques par run/tentative (`article1-attacks-rX-aY`, `article1-a10-rX`);
  - suppression non bloquante des namespaces de test apres chaque run;
  - suppression des CSV stale avant rerun;
  - `kubectl apply --validate=false` pour eviter que les timeouts OpenAPI client soient classes comme admission reelle.

## Reprise conseillee plus tard

Recreer AKS, redeployer `0.5.11`, verifier l'attestation reelle, puis relancer:

```bash
export KUBECONFIG=automation/terraform/aks-confidential/kubeconfig-aks

KUBECONFIG=automation/terraform/aks-confidential/kubeconfig-aks \
ENV_NAME=aks-real-sevsnp PLATFORM_NS=ai-platform OUT_DIR=article1/results/raw/aks \
N_RUNS_SECURITY=30 RUN_RETRIES=6 KUBECTL_RETRIES=10 KUBECTL_RETRY_SLEEP=3 \
REQUIRED_TEE=SEV-SNP RUNTIME_CLASS=runc REQUIRE_CONFIDENTIAL_CONTAINERS=false \
TOLERATE_CONFIDENTIAL_NODES=true EXPECTED_EVIDENCE_MODE=real \
bash article1/experiments/harness/run_security_campaign.sh
```

Ne pas utiliser `kind` comme resultat d'evaluation securite dans le papier principal. `kind` reste CI, debug et regression uniquement.

## Suppression AKS

- Suppression demandee par l'utilisateur.
- Action prevue: suppression du resource group `rg-article1-confidential`, qui ne contenait que `aks-article1`.
- Statut au moment de creation de ce fichier: suppression pas encore lancee.
