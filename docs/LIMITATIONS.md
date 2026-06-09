# Limites & hypothèses du MVP

## Périmètre assumé
- **Enforcement (slices 1 & 2) : décision, notification ET actuation réelle du reroute.** Selon
  `AISovereigntyPolicy.enforcementMode`, l'opérateur **agit** : `reportOnly` (constat seul), `warn`
  (alerte différenciée — Events Kubernetes + métrique `ai_finops_enforcement_actions`, sans blocage),
  `enforce` (**reroute réellement** le trafic du modèle non conforme vers le backend conforme dans le
  plan de données **Envoy AI Gateway** — mutation réversible de l'`AIGatewayRoute` : backend + réécriture
  du `model` ; `actuated=true`). Revert automatique au retour en `reportOnly`/`warn` ou à la suppression
  (finalizer). **Limites actuelles** : l'action `block` (modèle interdit *sans* fallback conforme) reste
  décidée mais non actuée ; le reroute est **par modèle** (pas par namespace/app — le routage gateway
  matche sur `x-ai-eg-model`) ; l'enforcement **budget** reste en recommandation (prochaine itération).
- **Pas d'attestation juridique.** Le produit prépare un dossier d'audit (RGPD, AI Act, politiques
  internes) ; il ne garantit pas la conformité.

## Données & calculs
- **Télémétrie** : `aigw` (Envoy AI Gateway / OpenTelemetry — **chemin réel**), `prometheus` et
  `configmap` opérationnels ; `fake` réservé à l'opt-in explicite. **Aucun repli `fake` silencieux** :
  sans source réelle, l'opérateur remonte `NoTelemetrySource` au lieu d'inventer des chiffres. Le mode
  télémétrie `litellm` (stub) a été retiré. À durcir : latence/erreurs et détection de reset des
  compteurs gateway (la prévision mensuelle est faussée juste après un redémarrage de la gateway).
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
