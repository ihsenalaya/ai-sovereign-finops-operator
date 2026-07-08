# GitOps Flow

Principe:
- images construites et référencées via `ghcr.io`
- manifests/Helm versionnés dans le fork privé
- aucun secret en clair dans le dépôt
- aucune dépendance à `ACR`

Flux:
1. build image
2. push vers `ghcr.io`
3. mettre à jour la référence d'image
4. déployer via GitOps
