#!/usr/bin/env bash
# aks-cost-check.sh — Article 1 AKS cost estimate (REAL prices, no resources).
#
# Queries the PUBLIC Azure Retail Prices API (no auth, no resource creation) for
# the intended confidential node SKU and prints an hourly/daily estimate for the
# planned article-1 topology. If the price cannot be fetched, it says so rather
# than inventing a number (prompt1.txt: never fabricate).
#
# Planned topology (article 1, <= 32 DCasv6 vCPUs):
#   - confidential pool: 1x Standard_DC8as_v6 (8 vCPU SEV-SNP)  [min=0 autoscale]
#   - system pool:       1x Standard_D2s_v3   (2 vCPU, non-confidential)
set -uo pipefail

CONF_SKU="${AKS_CONFIDENTIAL_SKU_PRIMARY:-Standard_DC8as_v6}"
SYS_SKU="${AKS_SYSTEM_SKU:-Standard_D2s_v3}"
OUT_DIR="${OUT_DIR:-article1/results/raw/aks}"
mkdir -p "${OUT_DIR}"

arm_region="${AZURE_REGION:-westus2}"

price_for() { # armSkuName -> hourly USD linux consumption price, or empty
  local sku="$1"
  local url="https://prices.azure.com/api/retail/prices?\$filter=armRegionName eq '${arm_region}' and armSkuName eq '${sku}' and priceType eq 'Consumption'"
  curl -fsS --max-time 25 "${url}" 2>/dev/null | python3 -c "
import json,sys
try:
    d=json.load(sys.stdin)
except Exception:
    sys.exit(0)
best=None
for it in d.get('Items',[]):
    name=(it.get('productName','')+' '+it.get('skuName','')+' '+it.get('meterName','')).lower()
    if 'windows' in name: continue
    if 'spot' in name or 'low priority' in name: continue
    p=it.get('retailPrice')
    if p and (best is None or p<best): best=p
if best is not None: print(best)
"
}

echo "== AKS cost estimate (article1) — region=${arm_region}, no resources created"
conf_price="$(price_for "${CONF_SKU}")"
sys_price="$(price_for "${SYS_SKU}")"

: > "${OUT_DIR}/cost-estimate.txt"
emit() { echo "$1" | tee -a "${OUT_DIR}/cost-estimate.txt"; }
emit "AKS cost estimate — $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
emit "region=${arm_region}"

if [ -n "${conf_price}" ]; then
  emit "confidential ${CONF_SKU}: \$${conf_price}/hr (Linux consumption)"
else
  emit "confidential ${CONF_SKU}: PRICE UNAVAILABLE (retail API not reachable) — do NOT guess"
fi
if [ -n "${sys_price}" ]; then
  emit "system       ${SYS_SKU}: \$${sys_price}/hr (Linux consumption)"
else
  emit "system       ${SYS_SKU}: PRICE UNAVAILABLE (retail API not reachable) — do NOT guess"
fi

if [ -n "${conf_price}" ] && [ -n "${sys_price}" ]; then
  python3 - "$conf_price" "$sys_price" <<'PY' | tee -a "${OUT_DIR}/cost-estimate.txt"
import sys
conf=float(sys.argv[1]); sysp=float(sys.argv[2])
hourly=conf+sysp
print(f"combined (1 conf + 1 system): ${hourly:.4f}/hr  ~ ${hourly*24:.2f}/day  ~ ${hourly*4:.2f} per 4h experiment window")
print("NOTE: AKS control plane (Free tier) = $0; egress, managed disks, and private registry traffic are not included.")
PY
else
  echo "combined estimate: UNAVAILABLE (a price could not be fetched)" | tee -a "${OUT_DIR}/cost-estimate.txt"
fi
echo "== cost estimate written to ${OUT_DIR}/cost-estimate.txt (no resources created)"
