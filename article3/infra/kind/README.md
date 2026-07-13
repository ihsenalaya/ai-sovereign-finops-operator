# Article 3 Kind infrastructure

This directory owns exactly three profiles and cluster names:

| Profile | Cluster | Nodes | CNI |
|---|---|---:|---|
| `dev` | `article3-dev` | 2 | Kind v0.31.0 kindnet |
| `validation` | `article3-validation` | 3 | Calico OSS v3.32.1 |
| `performance` | `article3-performance` | 4 | Calico OSS v3.32.1 |

Every profile uses the Kind v0.31.0 Kubernetes v1.35.0 node image pinned by
digest. Validation and performance disable Kind's default CNI and install the
official Calico v3.32.1 manifest only after its SHA-256 is verified. The script
then replaces all three Calico image tags with recorded multi-architecture
digests and verifies the transformed manifest hash before applying it. Before
the manifest is applied, every node pulls each exact digest through CRI with
bounded retry and verifies the expected repository digest without a generated
import alias. Raw containerd archive import is not used because containerd 2.2
can retain an invalid first-alias mapping in checkpoint-image metadata. Sources,
hashes, image digests, and retrieval date are in
`article3/provenance/kind_infrastructure.json`.

Use the profile, never an arbitrary cluster name:

```bash
PROFILE=dev bash article3/infra/kind/create.sh
PROFILE=validation bash article3/infra/kind/validate.sh
PROFILE=performance bash article3/infra/kind/collect-diagnostics.sh
PROFILE=dev bash article3/infra/kind/reset.sh
PROFILE=dev bash article3/infra/kind/destroy.sh
```

`install.sh` creates and validates infrastructure only. It deliberately does
not deploy GOV-AR while the Phase D implementation gate remains open.
`validate_release.sh` source-checks the automation, then idempotently creates
and validates all three exact profiles.

Performance creation fails closed unless `MemAvailable + SwapFree` is at least
2 GiB. This preflight prevents cumulative dev/validation/performance clusters
from exhausting a constrained Docker/WSL VM during CRI pulls. Operators may
raise the floor with `ARTICLE3_KIND_MIN_AVAILABLE_KIB`; lowering it is not part
of the reproducible validation workflow.

## Destructive-operation boundary

Creation writes a mode-0600 ownership record under
`${XDG_STATE_HOME:-$HOME/.local/state}/article3-q1-recovery/kind/`. Keeping this
capability record on the Linux state filesystem is required because a WSL
Windows-backed repository may not preserve restrictive POSIX modes. The record
binds the exact profile, manifest hash,
node image, Docker node IDs, configured image, and Kind cluster label. Every
validate, diagnose, reset, and destroy operation recomputes that binding.
It also recomputes the expected CNI, Calico version, and pinned transformed
manifest hash. The state directory is mode `0700`; a mode-`0600` probe must
round-trip on its filesystem before an ownership record is accepted.
Missing, stale, symlinked, permissive, or mismatched state fails closed. The
scripts reject all non-exact names, including pre-existing unrelated clusters,
and never read or change the current kubectl context.

Diagnostics are restricted to cluster-scoped infrastructure facts and the
`kube-system` namespace; they neither enumerate nor log application namespaces.
They exclude Kubernetes Secret objects and checksum every captured file. The
NetworkPolicy validation proves allow → deny → allow behavior using a
digest-pinned probe image; workload existence alone is not treated as evidence
of policy enforcement.
