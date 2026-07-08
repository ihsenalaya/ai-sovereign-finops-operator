# AKS Setup for Article 1

Statut actuel:
- région visée: `westus2`
- quota cible: `Standard DCasv6 Family vCPUs`, maximum 32 vCPU
- pas de GPU
- pas de TDX réel
- focus article 1: SEV-SNP node-level seulement si exécution réelle approuvée

Registry:
- utiliser `ghcr.io`
- ne pas utiliser `ACR`

Mode par défaut:
- `AZURE_APPLY_APPROVED=false`
- donc uniquement Terraform/docs/scripts/validation locale

Point à vérifier:
- compatibilité `Standard_DC8as_v6` avec l'isolation attendue
