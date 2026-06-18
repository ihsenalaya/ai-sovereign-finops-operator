# Limites & hypothèses du MVP

## Périmètre assumé
- **Enforcement : décision, notification ET actuation réelle dans Envoy AI Gateway.** Selon
  `AISovereigntyPolicy.enforcementMode`, l'opérateur **agit** : `reportOnly` (constat seul), `warn`
  (alerte différenciée — Events Kubernetes + métrique `ai_finops_enforcement_actions`, sans blocage),
  `enforce` (**reroute réellement** le trafic du modèle non conforme vers le backend conforme quand il
  existe, sinon **bloque réellement** la règle via le backend réservé absent `aiops-blocked`). Revert
  automatique au retour en `reportOnly`/`warn` ou à la suppression (finalizer). **Limite actuelle** : le
  contrôle reste **par modèle** (pas par namespace/app — le routage gateway matche sur `x-ai-eg-model`).
- **Enforcement budget** : le fallback managé est réellement actué depuis `AIBudgetPolicy`, mais avec
  un périmètre volontairement prudent : uniquement vers un `AIProvider.managed=true`, uniquement si le
  fallback est réellement moins cher sur le mix de tokens observé, uniquement sur des modèles **non
  partagés** hors cible, et jamais si une policy de souveraineté est déjà en `enforce`. Les garde-fous
  latence/erreur exigent une télémétrie qui expose ces signaux ; `aigw` porte désormais la latence
  observée via `gen_ai_server_request_duration_seconds`, tandis que les erreurs restent dépendantes
  d'une source qui les expose explicitement.
- **Pas d'attestation juridique.** Le produit prépare un dossier d'audit (RGPD, AI Act, politiques
  internes) ; il ne garantit pas la conformité.

## Données & calculs
- **Télémétrie** : `aigw` (Envoy AI Gateway / OpenTelemetry — **chemin réel**), `prometheus` et
  `configmap` opérationnels ; `fake` réservé à l'opt-in explicite. **Aucun repli `fake` silencieux** :
  sans source réelle, l'opérateur remonte `NoTelemetrySource` au lieu d'inventer des chiffres. Le mode
  télémétrie `litellm` (stub) a été retiré. La latence n'est marquée disponible que si elle est
  réellement observée ; à durcir : erreurs et détection de reset des compteurs gateway (la prévision
  mensuelle est faussée juste après un redémarrage de la gateway).
- **Coûts** : un [catalogue par défaut intégré](features/catalog.md) fournit prix + zone des
  modèles publics connus (OpenAI/Anthropic/Mistral/Gemini), donc le coût se calcule sans déclarer
  d'`AIProvider` ; un `AIProvider`/`AIModel` utilisateur **override** ces défauts. Un modèle
  **inconnu du catalogue par défaut ET sans CR** reste non pricé (recommandation data-quality).
  Les prix par défaut sont des **tarifs de liste publics** (best-effort, `PriceDate=2026-01`), pas
  une source de facturation.
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
- Manager non-root, RBAC minimal généré ; écriture limitée aux objets pilotés par l'opérateur
  (`AIGatewayRoute`, webhook config, Events, status/CRDs). Secrets de gateway référencés seulement
  (jamais copiés en status).
- Métriques exposées en clair (`:8080`) en MVP ; `--metrics-secure` disponible pour durcir.

Voir la [Roadmap](ROADMAP.md) pour le post-MVP.
