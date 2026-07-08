# Evaluation Plan

Expériences:
1. Security effectiveness
2. Scheduling overhead
3. Scalability/regression stress on `kind` and `kwok` (artifact-only, not main-paper evidence)
4. Race and freshness robustness
5. Ablation study

Baselines:
- B1 Kubernetes standard
- B2 node labels / nodeSelector
- B3 RuntimeClass only
- B4 scheduling gate + external controller
- B5 attestation-aware scheduler proposé

Règles méthodologiques:
- `N >= 10` par configuration
- warm-up exclu des statistiques mais conservé
- seeds documentés
- médiane, p95, p99, IC 95 %
- données brutes JSON par run
- environnement indiqué explicitement: `kind`, `kwok`, `AKS`
- résultats du papier principal: `AKS` réel SEV-SNP uniquement

Contraintes d'interprétation:
- toute expérience `kind` doit être marquée `SIMULATED — KIND ONLY — NOT REAL TEE/GPU`
- toute expérience `kind`/`kwok` est CI/debug/régression uniquement pour Article 1
- si une mesure AKS manque, écrire `NOT_EXECUTED` ou `PENDING_AKS_RERUN`
- pas de revendication de GPU confidentiel
- pas de revendication de TDX réel
