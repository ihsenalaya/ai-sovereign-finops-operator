# Reproducibility Checklist

- code source versionné localement
- cibles `make article1-*`
- registry d'images `ghcr.io`
- configuration `kind` documentée
- résultats principaux Article 1 issus d'AKS réel SEV-SNP uniquement
- `kind`/`kwok` explicitement regression-only
- seeds et paramètres d'expérience tracés
- résultats bruts non modifiés après collecte
- environnement de chaque figure explicitement indiqué
- aucune dépendance à `ACR`
