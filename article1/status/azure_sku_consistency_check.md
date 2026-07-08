# Azure SKU Consistency Check — Article 1

Generated: 2026-07-05

## Current Evaluation SKU

```text
region: westus2
confidential_vm_size: Standard_DC8as_v6
quota family: Standard DCasv6 Family vCPUs
quota used for full campaign: 4 x 8 vCPU = 32 vCPU
node-level TEE: AMD SEV-SNP Confidential VM
runtimeClassName used by Article 1 harness: runc
workloadRuntime: null
```

## Historical Failed Path

eastus2 / DCASv5 appears in older logs as a failed or non-exposed SKU path. It is
historical diagnostic evidence only and must not be described as the final
evaluation platform.

## Kata / Pod Sandboxing

AKS exposes `RuntimeClass/kata-vm-isolation`, but the Article 1 DCasv6 pool is
not a Kata workload-runtime pool. Attempts to use `KataMshvVmIsolation` on
`Standard_DC8as_v6` were rejected by AKS because nested virtualization is
required.

## Required Paper Language

Use:

```text
AKS westus2 Standard_DC8as_v6 AMD SEV-SNP confidential VM node-level attestation.
```

Do not use:

```text
pod-level Kata isolation on DCasv6
Confidential Containers evaluation
TDX evaluation
DCASv5 final evaluation
```

## Verdict

`PASS_WITH_CAVEAT`: SKU naming is now technically understood, but old generated
artifacts and text must remain clearly historical if retained.
