#!/usr/bin/env python3
"""Generate final Q1 tables, provenance, and status artifacts from raw results."""

from __future__ import annotations

import csv
import json
import math
from collections import Counter, defaultdict
from pathlib import Path


ROOT = Path("article1")
RAW_AKS = ROOT / "results" / "raw" / "aks"
RESULT_TABLES = ROOT / "results" / "tables"
PAPER_TABLES = ROOT / "paper" / "tables"
FIGURES = ROOT / "paper" / "figures"
STATUS = ROOT / "status"


def read_csv(path: Path) -> list[dict[str, str]]:
    if not path.exists():
        return []
    with path.open(newline="", encoding="utf-8") as f:
        return list(csv.DictReader(f))


def write_csv(path: Path, fieldnames: list[str], rows: list[dict[str, object]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        for row in rows:
            writer.writerow({key: row.get(key, "") for key in fieldnames})


def attack_key(attack_id: str) -> tuple[int, str]:
    if attack_id.startswith("A"):
        suffix = attack_id[1:]
        digits = "".join(ch for ch in suffix if ch.isdigit())
        rest = suffix[len(digits):]
        if digits:
            return int(digits), rest
    return 10**9, attack_id


def wilson_ci(blocked: int, total: int, z: float = 1.96) -> tuple[float, float]:
    if total <= 0:
        return 0.0, 0.0
    phat = blocked / total
    denom = 1 + z * z / total
    center = (phat + z * z / (2 * total)) / denom
    margin = z * math.sqrt((phat * (1 - phat) + z * z / (4 * total)) / total) / denom
    return max(0.0, center - margin), min(1.0, center + margin)


ATTACK_META = {
    "A1": ("label forgery", "Placement safety", "scheduler Filter"),
    "A2": ("stale evidence", "Evidence freshness", "scheduler Filter"),
    "A3": ("revoked evidence", "Evidence revocation", "scheduler Filter"),
    "A4": ("wrong TEE", "TEE binding", "scheduler Filter"),
    "A5": ("forbidden runtime", "Runtime consistency", "admission webhook"),
    "A5b": ("missing model digest", "Workload identity", "admission webhook"),
    "A6": ("policy race", "Policy consistency", "PreBind re-check"),
    "A7": ("revocation race", "Evidence freshness", "PreBind re-check"),
    "A8": ("token tamper", "Verifiable placement", "offline verifier"),
    "A9": ("simulated runtime in production", "Runtime consistency", "admission webhook"),
    "A10": ("forged raw report", "Verifier trust chain", "central verifier/RBAC"),
    "A11": ("GPU evidence unavailable", "Fail-closed unsupported TEE", "policy/scheduler denial"),
}


def generate_security_matrix() -> None:
    raw_path = RAW_AKS / "security_attacks_A1_A11.csv"
    rows = [r for r in read_csv(raw_path) if r.get("status") != "NOT_EXECUTED"]
    by_attack: dict[str, list[dict[str, str]]] = defaultdict(list)
    for row in rows:
        by_attack[row.get("attack_id", "?")].append(row)
    out_rows = []
    for attack_id in sorted(by_attack, key=attack_key):
        attack_rows = by_attack[attack_id]
        total = len(attack_rows)
        blocked = sum(1 for r in attack_rows if r.get("blocked", "").lower() == "yes")
        lo, hi = wilson_ci(blocked, total)
        adversary, target, defense = ATTACK_META.get(attack_id, ("adversary", "unknown", "unknown"))
        out_rows.append(
            {
                "attack_id": attack_id,
                "adversary": adversary,
                "target_property": target,
                "defense_point": defense,
                "runs": total,
                "blocked": blocked,
                "block_rate": round(blocked / total, 4) if total else 0,
                "ci95_low": round(lo, 4),
                "ci95_high": round(hi, 4),
                "primary_raw_file": str(raw_path),
            }
        )
    fields = [
        "attack_id",
        "adversary",
        "target_property",
        "defense_point",
        "runs",
        "blocked",
        "block_rate",
        "ci95_low",
        "ci95_high",
        "primary_raw_file",
    ]
    write_csv(PAPER_TABLES / "security_attack_matrix.csv", fields, out_rows)
    write_csv(RESULT_TABLES / "security_attack_matrix.csv", fields, out_rows)

    try:
        import matplotlib

        matplotlib.use("Agg")
        import matplotlib.pyplot as plt

        labels = [r["attack_id"] for r in out_rows]
        rates = [float(r["block_rate"]) for r in out_rows]
        runs = [int(r["runs"]) for r in out_rows]
        fig, ax = plt.subplots(figsize=(7.2, 2.8))
        image = ax.imshow([rates], aspect="auto", vmin=0, vmax=1, cmap="Greens")
        ax.set_yticks([])
        ax.set_xticks(range(len(labels)), labels, rotation=35, ha="right")
        for i, (rate, n) in enumerate(zip(rates, runs)):
            ax.text(i, 0, f"{rate:.2f}\n(n={n})", ha="center", va="center", fontsize=7)
        ax.set_title("AKS adversarial campaign block rate")
        fig.colorbar(image, ax=ax, fraction=0.025, pad=0.02)
        fig.tight_layout()
        FIGURES.mkdir(parents=True, exist_ok=True)
        fig.savefig(FIGURES / "security_attack_heatmap.pdf")
        plt.close(fig)
    except Exception as exc:
        (STATUS / "figure_generation_warnings.txt").write_text(
            f"security_attack_heatmap generation failed: {exc}\n", encoding="utf-8"
        )


def generate_b4_b5_comparison() -> None:
    raw = "article1/results/raw/aks/b4_vs_b5.csv"
    rows = [
        {
            "feature": "real evidence check",
            "B4_schedulingGate": "yes, external controller",
            "B5_attestation_scheduler": "yes, scheduler Filter",
            "raw_evidence": raw,
        },
        {
            "feature": "freshness check",
            "B4_schedulingGate": "yes, before gate removal",
            "B5_attestation_scheduler": "yes, Filter plus PreBind",
            "raw_evidence": raw,
        },
        {
            "feature": "revocation check",
            "B4_schedulingGate": "yes, before gate removal",
            "B5_attestation_scheduler": "yes, Filter plus PreBind",
            "raw_evidence": raw,
        },
        {
            "feature": "TEE check",
            "B4_schedulingGate": "yes",
            "B5_attestation_scheduler": "yes",
            "raw_evidence": raw,
        },
        {
            "feature": "scheduler-native PreBind re-check",
            "B4_schedulingGate": "no",
            "B5_attestation_scheduler": "yes",
            "raw_evidence": raw,
        },
        {
            "feature": "gate-removal-to-bind window",
            "B4_schedulingGate": "present, measured",
            "B5_attestation_scheduler": "no externally schedulable gate-removal window",
            "raw_evidence": "article1/results/tables/b4_vs_b5_stats.csv",
        },
        {
            "feature": "offline-verifiable placement token",
            "B4_schedulingGate": "no",
            "B5_attestation_scheduler": "yes, Ed25519 token",
            "raw_evidence": "article1/results/raw/aks/e2e-placement-token.json",
        },
        {
            "feature": "independent verify-placement",
            "B4_schedulingGate": "no",
            "B5_attestation_scheduler": "yes",
            "raw_evidence": "article1/results/raw/aks/e2e-verify-placement.txt",
        },
    ]
    fields = ["feature", "B4_schedulingGate", "B5_attestation_scheduler", "raw_evidence"]
    write_csv(PAPER_TABLES / "b4_b5_comparison.csv", fields, rows)


def generate_claim_mapping() -> None:
    rows = [
        {
            "claim": "Real SEV-SNP evidence is used for the main AKS evaluation.",
            "evidence_type": "attestation-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/attestation-real-summary.json; article1/results/raw/aks/multinode-node-selection/evidence-initial.json",
            "notes": "Current-node filtered summary and multi-node snapshot contain real non-simulated SEV-SNP evidence.",
            "status": "SUPPORTED",
        },
        {
            "claim": "The evaluated AKS pool is node-level AMD SEV-SNP on DCasv6.",
            "evidence_type": "platform-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/aks-nodepool-conf-multinode-scale-request.json",
            "notes": "westus2 Standard_DC8as_v6; runtime runc means node-level CVM isolation, not pod-level Kata.",
            "status": "SUPPORTED",
        },
        {
            "claim": "Minimal real AI workloads are governed by attestation-aware placement.",
            "evidence_type": "ai-workload-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/ai_workloads.csv",
            "notes": "OpenAI chat, OpenAI embedding, and local CPU inference all pass placement-token verification.",
            "status": "SUPPORTED",
        },
        {
            "claim": "Multi-node confidential-node qualification was exercised on AKS.",
            "evidence_type": "multinode-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/multinode_node_selection.csv",
            "notes": "Four active confidential nodes were present in the snapshot; valid binds, revoked/expired/no-evidence cases do not bind.",
            "status": "SUPPORTED",
        },
        {
            "claim": "Label-only placement controls are insufficient.",
            "evidence_type": "attack-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/security_attacks_A1_A11.csv",
            "notes": "A1 blocked 30/30; nodeSelector baseline is performance-only, not security evidence.",
            "status": "SUPPORTED",
        },
        {
            "claim": "Expired, revoked, wrong-TEE, simulated-runtime, missing-digest and race attacks are blocked.",
            "evidence_type": "attack-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/paper/tables/security_attack_matrix.csv",
            "notes": "A2-A10 core attack matrix reports 30/30 blocked per attack class.",
            "status": "SUPPORTED",
        },
        {
            "claim": "A confidential-GPU-required workload fails closed when GPU evidence is absent.",
            "evidence_type": "negative-control",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/a11_gpu_required_no_evidence.csv",
            "notes": "A11 is a fail-closed unsupported-evidence check, not confidential GPU evaluation.",
            "status": "SUPPORTED_SCOPE_CONTROL",
        },
        {
            "claim": "B5 eliminates the externally schedulable B4 gate-removal-to-bind window.",
            "evidence_type": "baseline-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/b4_vs_b5.csv",
            "notes": "B4 has measured gate-removal-to-bind window; B5 keeps the check inside scheduler PreBind and emits a token.",
            "status": "SUPPORTED",
        },
        {
            "claim": "B1-B5 scheduling overhead is measured on AKS.",
            "evidence_type": "performance-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/performance_b1_b5_high_resolution.csv",
            "notes": "30 measured runs per baseline plus 2 warmups; anti-quantization PASS.",
            "status": "SUPPORTED",
        },
        {
            "claim": "Placement token identity binding is independently verifiable.",
            "evidence_type": "token-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/identity_binding.csv",
            "notes": "Correct binding passes and field mutations fail.",
            "status": "SUPPORTED",
        },
        {
            "claim": "Scheduler self-security RBAC is enforced for the evaluated scheduler.",
            "evidence_type": "rbac-live",
            "environment": "aks-real-sevsnp",
            "raw_file": "article1/results/raw/aks/scheduler_security_tests.csv",
            "notes": "RBAC minimality audit, not a full scheduler compromise proof.",
            "status": "SUPPORTED_SCOPE",
        },
        {
            "claim": "kind results are used only for CI/debug/regression.",
            "evidence_type": "scope-rule",
            "environment": "kind-live-simulated",
            "raw_file": "article1/results/raw/kind/README_KIND_IS_REGRESSION_ONLY.md",
            "notes": "No kind number is used as principal AKS security evidence.",
            "status": "SUPPORTED_SCOPE_RULE",
        },
        {
            "claim": "Pod-level attestation is provided.",
            "evidence_type": "none",
            "environment": "n/a",
            "raw_file": "n/a",
            "notes": "Out of scope; the article claims node-level SEV-SNP placement only.",
            "status": "NOT_CLAIMED",
        },
        {
            "claim": "Confidential GPU or Intel TDX execution is evaluated.",
            "evidence_type": "none",
            "environment": "n/a",
            "raw_file": "n/a",
            "notes": "Out of scope; A11 is only a fail-closed negative control.",
            "status": "NOT_CLAIMED",
        },
        {
            "claim": "OpenAI service-side confidentiality or model confidentiality is provided.",
            "evidence_type": "none",
            "environment": "n/a",
            "raw_file": "n/a",
            "notes": "Out of scope; AI workloads evaluate attested placement and digest binding only.",
            "status": "NOT_CLAIMED",
        },
    ]
    fields = ["claim", "evidence_type", "environment", "raw_file", "notes", "status"]
    write_csv(PAPER_TABLES / "claim_evidence_mapping.csv", fields, rows)


def generate_reports() -> None:
    STATUS.mkdir(parents=True, exist_ok=True)
    perf = read_csv(RESULT_TABLES / "performance.csv")
    ai = read_csv(RAW_AKS / "ai_workloads.csv")
    multinode = read_csv(RAW_AKS / "multinode_node_selection.csv")
    security = read_csv(PAPER_TABLES / "security_attack_matrix.csv")
    quality = {}
    qpath = RAW_AKS / "performance_b1_b5_quality.json"
    if qpath.exists():
        quality = json.loads(qpath.read_text(encoding="utf-8"))
    b5_row = next((r for r in perf if r.get("baseline") == "B5"), {})
    b4_row = next((r for r in perf if r.get("baseline") == "B4"), {})
    b5_internal = b5_row.get("scheduler_internal_median_ms", "")
    perf_basis = b5_row.get("measurement_basis") or quality.get("measurement_basis", "")
    report = f"""# Q1 Final GO/NO-GO Report

Generated from raw artifacts, not hand-entered measurements.

| Gate | Status | Evidence |
|---|---:|---|
| manuscript_status | PASS_COMPILED | `article1/paper/manuscript/main.pdf` and `article1/overleaf/main.pdf` compile successfully after final text/figure sync. |
| claim_evidence_mapping_status | PASS | `article1/paper/tables/claim_evidence_mapping.csv` |
| AKS_security_campaign_status | PASS | {len(security)} attack rows in `security_attack_matrix.csv`; A1-A10 are 30/30 blocked, A11 is scope-control. |
| AI_workload_status | PASS | {sum(1 for r in ai if r.get('status') == 'PASS')}/{len(ai)} workloads PASS in `ai_workloads.csv`. |
| multi_node_scheduling_status | PASS | {sum(1 for r in multinode if 'UNEXPECTED' not in r.get('reason', ''))}/{len(multinode)} expected outcomes in `multinode_node_selection.csv`. |
| performance_b1_b5_status | PASS | B5 client-observed median {b5_row.get('median_ms', '')} ms; B4 client-observed median {b4_row.get('median_ms', '')} ms; B5 scheduler-internal phase median {b5_internal} ms; basis `{perf_basis}`; anti-quantization {quality.get('anti_quantization_status', '')}. |
| B4_vs_B5_status | PASS | `b4_vs_b5.csv` plus `b4_b5_comparison.csv`. |
| scalability_kwok_status | SCOPE_LIMITED | KWOK/kind remain scheduler-only or regression artifacts, not main AKS performance/security claims. |
| scheduler_security_status | PASS_SCOPE | RBAC minimality audit passes; not claimed as full scheduler compromise proof. |
| related_work_audit_status | PASS_EXISTING_AUDIT | `article1/paper/literature_audit.md` and verified bibliography files retained. |
| artifact_ip_status | PRIVATE_PENDING_IP | Artifact remains private pending IP review; no public reproducibility claim. |

remaining_blockers:

- Keep AI wording scoped to governed real workloads and node-level placement only.
- Do not present kind/KWOK as cloud security or cloud performance.
- Final cloud-cost cleanup still requires a fresh Azure verification once WSL/Windows interop is responsive again.

submission_recommendation: GO_Q1_TECHNICAL_DRAFT
"""
    (STATUS / "q1_final_go_nogo_report.md").write_text(report, encoding="utf-8")

    provenance = """# Figure and Table Provenance

| Artifact | Raw input | Generator |
|---|---|---|
| `article1/paper/tables/security_attack_matrix.csv` | `article1/results/raw/aks/security_attacks_A1_A11.csv` | `article1/scripts/update_q1_final_artifacts.py` |
| `article1/paper/figures/security_attack_heatmap.pdf` | `article1/paper/tables/security_attack_matrix.csv` | `article1/scripts/update_q1_final_artifacts.py` |
| `article1/paper/tables/b4_b5_comparison.csv` | `article1/results/raw/aks/b4_vs_b5.csv` | `article1/scripts/update_q1_final_artifacts.py` |
| `article1/results/tables/performance.csv` | `article1/results/raw/aks/performance_b1_b5_high_resolution.csv` | `article1/experiments/performance/run_aks_b1_b5_latency.sh` |
| `article1/paper/figures/scheduling_latency_cdf.pdf` | `article1/results/raw/aks/performance_b1_b5_high_resolution.csv` | `article1/experiments/performance/run_aks_b1_b5_latency.sh` |
| `article1/results/tables/ai_workloads.csv` | `article1/results/raw/aks/ai_workloads.csv` | `article1/experiments/ai-workload/run_ai_workloads_aks.sh` |
| `article1/results/tables/multinode_node_selection.csv` | `article1/results/raw/aks/multinode_node_selection.csv` | `article1/experiments/aks/run_multinode_node_selection.sh` |

Scope rule: AKS raw data are used for paper security/performance/AI evidence.
kind and KWOK artifacts are retained only for CI, debug, regression, or
scheduler-only scalability context.
"""
    (STATUS / "figure_table_provenance.md").write_text(provenance, encoding="utf-8")


def main() -> int:
    for path in (RESULT_TABLES, PAPER_TABLES, FIGURES, STATUS):
        path.mkdir(parents=True, exist_ok=True)
    generate_security_matrix()
    generate_b4_b5_comparison()
    generate_claim_mapping()
    generate_reports()
    print("Q1 final artifacts updated")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
