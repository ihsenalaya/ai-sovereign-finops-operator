# Article 1 Implementation Plan

Titre de travail: `Attestation-Aware Kubernetes Scheduling for Confidential AI Inference`

Objectif:
- démontrer un placement Kubernetes vérifiable pour charges IA sensibles;
- distinguer strictement simulation `kind` et exécution réelle;
- livrer un artefact reproductible sans revendiquer de GPU confidentiel ni de TDX réel.

Périmètre article 1:
- `ConfidentialInferencePolicy`
- `AttestationEvidence`
- `AIPlacementDecision`
- webhook de mutation/validation
- scheduler `ai-attestation-scheduler`
- placement token minimal Ed25519
- CLI indépendante `verify-placement`
- évaluation principale AKS réel SEV-SNP uniquement
- `kind` + `kwok` conservés pour CI/debug/régression, jamais comme preuve principale
- AKS westus2 DCasv6 utilisé pour les résultats empiriques déjà collectés

Écarts critiques à fermer:
1. durcissement `PreBind` fail-closed
2. attente bornée de disponibilité d'evidence
3. hashing déterministe canonique
4. tests scheduler couvrant freshness, révocation et courses
5. package article reproductible et honnête

Choix d'artefact:
- registry d'images: `ghcr.io`
- aucune dépendance à `ACR`
- aucune affirmation de résultat réel sans trace brute associée
