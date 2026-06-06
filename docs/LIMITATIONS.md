# Limites & hypothèses du MVP

## Périmètre assumé
- **Lecture seule & non-bloquant.** L'opérateur n'altère jamais la gateway ni les workloads.
  `AISovereigntyPolicy.enforcementMode` = `reportOnly` ; budgets et souveraineté sont en
  **recommandation uniquement**. Les modes `warn`/`enforce` sont post-MVP.
- **Pas d'attestation juridique.** Le produit prépare un dossier d'audit (RGPD, AI Act, politiques
  internes) ; il ne garantit pas la conformité.

## Données & calculs
- **Télémétrie** : collector `fake` (démo) et `prometheus` opérationnels ; **LiteLLM** est un stub ;
  **Envoy/OTel** à venir.
- **Coûts** : basés sur le pricing `AIProvider` par million de tokens ; modèles sans `AIProvider`
  comptés comme non pricés (recommandation data-quality).
- **Devise** : pas de conversion multi-devise ; la devise dominante est rapportée telle quelle.
- **Break-even** : modèle simple (extrapolation linéaire de l'usage observé ; coûts GPU/ops fournis
  par l'utilisateur). Seuil de payback par défaut : 6 mois.

## Plateforme
- Versions d'outils bumpées pour Go 1.25 (controller-gen 0.18, kustomize 5.4.3, envtest k8s 1.31).
- `image.pullPolicy=Never` réservé à kind (image chargée localement) ; sur un vrai cluster utiliser
  un registre.
- ArgoCD nécessite un dépôt Git accessible ; sans remote, utiliser le chemin Helm offline
  (`automatisation/scripts/install-local.sh`).

## Sécurité
- Manager non-root, RBAC minimal généré, secrets de gateway référencés (jamais copiés en status).
- Métriques exposées en clair (`:8080`) en MVP ; `--metrics-secure` disponible pour durcir.

Voir la [Roadmap](ROADMAP.md) pour le post-MVP.
