# Fonctionnalité — Plan de souveraineté eBPF / Shadow-AI (`internal/shadowengine`)

Plan de souveraineté **indépendant de la gateway**. Les collecteurs classiques ne voient que le
trafic qui **passe par** l'Envoy AI Gateway. Un pod qui appelle `api.openai.com` **en direct**
contourne tout — c'est le **shadow-AI**, le plus gros angle mort. Ce plan le ferme : eBPF
(Tetragon) observe l'egress par pod **dans le noyau**, et l'opérateur classe chaque destination
par **zone de souveraineté** via le [catalogue par défaut](catalog.md).

```
pod ──TLS──▶ api.openai.com (US)        ❌ ne touche jamais la gateway
  │
  └─ Tetragon (eBPF) voit le connect ──▶ forwarder ──▶ ConfigMap shadow-egress
                                                            │
                          opérateur : shadowengine.Detect( EndpointToZone )
                                                            │
                    ai_finops_shadow_ai_egress + panneaux Grafana "Shadow-AI egress"
```

> **No-fake.** La détection tourne sur des **connexions réellement observées** par eBPF, jamais
> fabriquées. Pur (aucune dépendance K8s/eBPF), unit-testé (`shadowengine_test.go`).

## API (pure)
```go
type Egress struct { Namespace, Application, Host string; Connections int64 }
type Finding struct { Namespace, Application, Host, Provider, Zone, Severity, Message string; Connections int64 }
type ZoneResolver func(host string) (zone, provider string)   // passer catalog.EndpointToZone

func Detect(policy sovereigntyengine.Policy, egresses []Egress, resolve ZoneResolver) []Finding
```

`Detect` ne garde que les egress vers un **endpoint LLM reconnu** (host inconnu → ignoré : c'est
de la détection d'egress IA, pas un pare-feu). **Zone interdite → critical**, hors zones
autorisées → warning, zone conforme → rien.

## Câblage — reconciler `AISovereigntyPolicy`
- Lit l'egress observé du ConfigMap conventionnel **`shadow-egress`** (clé `egress.json`, tableau
  de `{namespace,application,host,connections}`) dans le namespace de la policy. Absent → no-op
  (opt-in par la présence de **vraies** données eBPF).
- Tourne **tôt et inconditionnellement** — même **sans aucune AIGateway** (le cas shadow-AI
  précis : du trafic qui ne touche jamais la gateway).
- Émet `ai_finops_shadow_ai_egress{namespace,application,zone,provider,severity}` (tracker par-UID
  `shadowMetrics`, `forget` à la suppression de la policy) + Events Kubernetes `ShadowAI`.

## Source des données eBPF — `automatisation/tetragon/`
**Tetragon** (DaemonSet eBPF standalone — *pas* Cilium : aucun changement de CNI, tout cluster) :
- `install.sh` : Helm install + `tracingpolicy.yaml` (`tcp_connect` port 443).
- `forwarder.sh` : lit le **fichier d'export** Tetragon (`/var/run/cilium/tetragon/tetragon.log`),
  garde les `tcp_connect`, mappe l'IP destination → host LLM connu (résolution DNS des hosts du
  catalogue), agrège par `(namespace, workload, host)` et écrit le ConfigMap `shadow-egress`
  (python3 ; pas de dépendance `jq`).
- `rogue-app.yaml` : workload de démo qui appelle OpenAI **US** en direct.

**Validé live sur AKS** : `finance/shadow-ai-rogue → api.openai.com` ressort en
`ai_finops_shadow_ai_egress{zone="US",severity="critical"}`, visible sur le dashboard.

## Limites & évolutions
- **Mapping IP→host** : par résolution DNS des hosts connus (les CDN/fronting peuvent brouiller ;
  le SNI serait plus précis). L'egress par IP reste observé.
- **ECH** (Encrypted Client Hello) pourra éroder la visibilité SNI à terme ; l'observation
  IP/destination tient.
- Évolutions : backend gRPC Tetragon natif dans l'opérateur (au lieu du pont ConfigMap), backend
  **Hubble** (si Cilium est le CNI), capture SNI. Le design est *backend-agnostique* : tout ce qui
  remplit `shadow-egress` fonctionne.

Voir aussi : [catalog](catalog.md) (`EndpointToZone`), [sovereigntyengine](sovereigntyengine.md)
(plan gateway), [metrics](metrics.md), [DASHBOARDS](../DASHBOARDS.md).
