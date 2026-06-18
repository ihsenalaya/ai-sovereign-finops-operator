# Fonctionnalité — Enforcement engine (`internal/enforcementengine`)

Transforme l'`enforcementMode` d'une politique et les **violations observées** en **décisions
d'enforcement** concrètes. Moteur **pur** (aucune dépendance Kubernetes) : c'est le seul endroit qui
décide *quoi* faire d'une violation ; les controllers exécutent la décision (Events + métrique
aujourd'hui ; actuation au niveau de la gateway en slice 2). C'est le passage d'**observateur** à
**plan de contrôle**.

## Décision
```go
type Action string // report | warn | block | reroute

type Violation struct {
    Namespace, Application, Provider, Zone, Model string
    Requests                                      int64
}

type Decision struct {
    Mode      string  // reportOnly | warn | enforce
    Action    Action
    Namespace, Application, Provider, Zone, Model string
    Requests  int64
    RerouteTo string  // modèle conforme cible quand Action == reroute
    Message   string
    Actuated  bool    // true = exécuté de bout en bout aujourd'hui (report, warn)
}

func DecideSovereignty(mode string, violations []Violation, fallbackModel string) []Decision
```

## Règles (souveraineté)
| Mode | Action | `Actuated` | Détail |
|------|--------|-----------|--------|
| `reportOnly` | `report` | `true` | Le constat est enregistré, rien d'autre. |
| `warn` | `warn` | `true` | Alerte levée (Event + métrique) ; le trafic n'est pas touché. |
| `enforce` (+ fallback conforme) | `reroute` | `true`* | Reroute **réellement actué** vers le modèle conforme **le moins cher** (`fallbackModel`). |
| `enforce` (sans fallback) | `block` | `true`* | Blocage **réellement actué** en pointant la règle vers le backend réservé absent `aiops-blocked`. |

\* `Actuated` passe à `true` **uniquement** quand l'actuation gateway a réussi (route patchée).

## Actuation dans le plan de données — `gatewayactuator`
En mode `enforce`, le controller appelle l'actuateur qui **mute réellement** l'`AIGatewayRoute`
d'Envoy AI Gateway (client *unstructured*, aucune dépendance Go sur l'API Envoy AI Gateway) :
- `reroute` : la règle qui matche `x-ai-eg-model: <modèle interdit>` voit son `backendRefs[0].name`
  basculé vers le **backend conforme** (celui qui sert le modèle cible), et un `bodyMutation.set`
  réécrit le champ `model` du corps de requête vers le modèle conforme ;
- `block` : quand aucun modèle conforme n'existe, la règle est pointée vers le backend réservé absent
  `aiops-blocked`, ce qui empêche la résolution de backend et bloque l'appel au fournisseur interdit.

**Réversible** : le backend d'origine par modèle est stocké dans l'annotation
`aiops.imperium.io/enforced-reroutes` de la route. Le revert est automatique au retour en
`reportOnly`/`warn` (le `desired` devient vide) et à la **suppression de la policy** (le finalizer
restaure les routes avant de partir). RBAC requis : `aigateway.envoyproxy.io/aigatewayroutes`
(`get;list;watch;update;patch`).

## Câblage
Le controller `AISovereigntyPolicy` ([doc](../crds/aisovereigntypolicy.md)) construit les `Violation`
depuis les findings **critiques**, choisit le `fallbackModel` via `catalog.cheapestCompliantModelName`,
appelle `DecideSovereignty`, puis pour chaque décision : émet un **Event Kubernetes** différencié, pose
la série `ai_finops_enforcement_actions{policy,namespace,application,mode,action,actuated}` (purgée par un
tracker par-UID + finalizer, pas de série fantôme), et résume la posture dans la condition `Ready`.

Lié : [AISovereigntyPolicy](../crds/aisovereigntypolicy.md), [sovereigntyengine](sovereigntyengine.md),
[metrics](metrics.md).
