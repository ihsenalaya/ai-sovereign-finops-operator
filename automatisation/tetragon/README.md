# Shadow-AI sovereignty plane (Tetragon / eBPF)

The gateway-based collectors only see traffic that goes **through** the Envoy AI
Gateway. A pod that calls `api.openai.com` directly bypasses all of it — that's
**shadow-AI**, the biggest sovereignty blind spot. This plane closes it: Tetragon
observes per-pod egress in the kernel (eBPF), independent of any gateway, and the
greenops operator classifies each destination by sovereignty zone.

```
pod ──TLS──▶ api.openai.com (US)        ❌ never touches the gateway
  │
  └─ Tetragon (eBPF) sees the connect ──▶ forwarder ──▶ shadow-egress ConfigMap
                                                              │
                              operator: shadowengine.Detect( endpoint→zone )
                                                              │
                          ai_finops_shadow_ai_egress + Grafana "Shadow-AI egress"
```

This stays true to the project rule: **the data is real** (eBPF-observed
connections), never fabricated. No gateway, no SDK, no app cooperation required.

## Why Tetragon (not Cilium/Hubble)

Tetragon is a **standalone DaemonSet** — it works on **any** CNI with no cluster
re-creation. Hubble needs Cilium as the CNI (a disruptive swap). Both are CNCF /
Apache-2.0. The operator side is backend-agnostic: anything that fills the
`shadow-egress` ConfigMap works (Tetragon today, a native Hubble/Tetragon gRPC
backend later).

## Requirements

A cluster with a BPF-capable kernel + BTF (`/sys/kernel/btf/vmlinux`). **AKS** (or
any standard managed k8s) is ideal. A memory-starved kind/WSL usually lacks room for
the Tetragon DaemonSet — prefer AKS for this plane.

## Run it

```bash
# 1. Install Tetragon + the egress TracingPolicy
./install.sh

# 2. (demo) deploy a rogue workload that calls OpenAI US directly, bypassing the gateway
kubectl apply -f rogue-app.yaml

# 3. Bridge eBPF events -> the ConfigMap the operator reads (run periodically, e.g. cron)
NS=default ./forwarder.sh      # NS = the namespace of your AISovereigntyPolicy

# 4. Observe
#    - operator metric:  ai_finops_shadow_ai_egress{namespace,application,zone,provider,severity}
#    - Grafana:          "Shadow-AI egress (namespace → zone)" + "Shadow-AI egress by zone"
#    - K8s events:       kubectl describe aisovereigntypolicy <name>  (Warning/ShadowAI)
```

With an `EU`-allowed / `US`-forbidden policy, the rogue `finance/rogue-script` flow to
`api.openai.com` surfaces as a **critical** shadow-AI finding.

## Honest caveats

- **IP→host mapping.** `forwarder.sh` maps observed destination IPs to LLM hostnames
  by resolving the providers' own hostnames. Shared CDNs/fronting can blur this; SNI
  is more precise. A TLS-SNI TracingPolicy is the accurate upgrade (kept out here to
  stay simple and portable).
- **Event schema.** The `jq` paths follow Tetragon's `process_kprobe` schema; tune
  them to your Tetragon version if fields differ (`tetra getevents -o json` to inspect).
- **ECH.** SNI is cleartext today but Encrypted Client Hello may erode SNI-based
  visibility over time; IP/destination observation still holds.
