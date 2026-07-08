#!/usr/bin/env bash
# aks-preflight.sh — Article 1 AKS preflight (REAL checks, no resource creation).
#
# Verifies the mandatory preconditions from prompt1.txt §A4 BEFORE any Azure
# resource can be created:
#   1. an Azure account is logged in (does NOT print/persist the subscription ID)
#   2. region is westus2
#   3. Standard DCasv6 Family vCPUs quota >= required (default 32)
#   4. Total Regional vCPUs quota is sufficient
#
# This script creates NO resources. It exits non-zero if any gate fails, so a
# caller must STOP before terraform apply. Evidence (quota table) is written to
# ${OUT_DIR} WITHOUT the subscription ID.
set -euo pipefail

AZURE_REGION="${AZURE_REGION:-westus2}"
REQUIRED_DCASV6="${AZURE_CONFIDENTIAL_VCPU_QUOTA:-32}"
REQUIRED_TOTAL="${AKS_REQUIRED_TOTAL_VCPU:-40}"
OUT_DIR="${OUT_DIR:-article1/results/raw/aks}"
mkdir -p "${OUT_DIR}"

fail() { echo "PREFLIGHT FAIL: $*" >&2; exit 1; }

echo "== AKS preflight (article1) — region=${AZURE_REGION}, no resources created"

# Gate 1: logged in. Show only the subscription NAME, never the ID.
sub_name="$(az account show --query name -o tsv 2>/dev/null || true)"
[ -n "${sub_name}" ] || fail "not logged in (run 'az login')"
echo "  [1/4] azure login OK — subscription: ${sub_name}"

# Gate 2: region.
[ "${AZURE_REGION}" = "westus2" ] || fail "region must be westus2 (got ${AZURE_REGION})"
echo "  [2/4] region is westus2 OK"

# Gates 3+4: quota. Fetch once, parse for both.
usage_json="$(az vm list-usage --location "${AZURE_REGION}" -o json 2>/dev/null || true)"
[ -n "${usage_json}" ] || fail "could not read vm usage for ${AZURE_REGION}"
# Persist evidence WITHOUT subscription id.
echo "${usage_json}" > "${OUT_DIR}/vm-usage-${AZURE_REGION}.json"

read_limit() { # localizedName-substring
  echo "${usage_json}" | python3 -c "
import json,sys
key=sys.argv[1]
for u in json.load(sys.stdin):
    name=u.get('name',{}).get('localizedValue','') or u.get('localName','')
    if key.lower() in name.lower():
        print(int(u['currentValue']), int(u['limit'])); break
" "$1"
}

dcas="$(read_limit 'DCasv6 Family')"
[ -n "${dcas}" ] || fail "DCasv6 family quota not found in ${AZURE_REGION}"
dcas_cur="${dcas% *}"; dcas_lim="${dcas#* }"
echo "  [3/4] DCasv6 quota: current=${dcas_cur} limit=${dcas_lim} (need >= ${REQUIRED_DCASV6})"
[ "${dcas_lim}" -ge "${REQUIRED_DCASV6}" ] || fail "DCasv6 limit ${dcas_lim} < required ${REQUIRED_DCASV6}"

total="$(read_limit 'Total Regional vCPUs')"
[ -n "${total}" ] || fail "Total Regional vCPUs quota not found"
total_cur="${total% *}"; total_lim="${total#* }"
echo "  [4/4] Total Regional vCPUs: current=${total_cur} limit=${total_lim} (need >= ${REQUIRED_TOTAL})"
[ "${total_lim}" -ge "${REQUIRED_TOTAL}" ] || fail "Total Regional vCPU limit ${total_lim} < required ${REQUIRED_TOTAL}"

cat > "${OUT_DIR}/preflight-summary.txt" <<EOF
AKS preflight summary (article1) — $(date -u '+%Y-%m-%dT%H:%M:%SZ')
region=${AZURE_REGION}
subscription_name=${sub_name}
dcasv6_current=${dcas_cur} dcasv6_limit=${dcas_lim} required=${REQUIRED_DCASV6}
total_regional_current=${total_cur} total_regional_limit=${total_lim} required=${REQUIRED_TOTAL}
result=PASS
EOF

echo "== PREFLIGHT PASS — quota/region gates satisfied (evidence in ${OUT_DIR})"
echo "   NOTE: this does NOT create resources. terraform apply remains gated on"
echo "   explicit spend approval AND a fully-authored confidential-cluster module."
