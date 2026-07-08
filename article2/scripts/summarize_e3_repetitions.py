#!/usr/bin/env python3
import argparse
import json
import math
import pathlib
import re
from collections import defaultdict


REP_RE = re.compile(r"-(r\d{2})$")


def mean(values):
    return sum(values) / len(values) if values else 0.0


def stddev(values):
    if len(values) <= 1:
        return 0.0
    mu = mean(values)
    return math.sqrt(sum((v - mu) ** 2 for v in values) / len(values))


def base_name(name: str) -> str:
    return REP_RE.sub("", name)


def stability(verdicts):
    uniq = sorted(set(v for v in verdicts if v))
    if not uniq:
        return "no-verdict"
    if len(uniq) == 1:
        if uniq[0] == "candidate-safe":
            return "stable-safe"
        if uniq[0] == "candidate-risk":
            return "stable-risk"
        return f"stable-{uniq[0]}"
    return "unstable"


def target_label(item):
    spec = item.get("spec", {})
    target = spec.get("target", {})
    return f"{target.get('namespace','')}/{target.get('application','')}"


def render_tex(rows):
    lines = [
        r"\begin{tabular}{p{3.1cm} p{2.5cm} p{1.5cm} p{1.5cm} p{1.0cm} p{1.0cm} p{0.8cm} p{0.9cm}}",
        r"\toprule",
        r"Gate & Target & Stability & Verdict mix & Mean & Std & N & Reps \\",
        r"\midrule",
    ]
    for row in rows:
        lines.append(
            f"{row['gate']} & {row['target']} & {row['stability']} & {row['verdict_mix']} & "
            f"{row['mean_score']:.3f} & {row['std_score']:.3f} & {row['samples']} & {row['repetitions']} \\\\"
        )
    lines.extend([r"\bottomrule", r"\end{tabular}"])
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input-json", required=True)
    parser.add_argument("--output-json", required=True)
    parser.add_argument("--output-md", required=True)
    parser.add_argument("--output-tex", required=True)
    args = parser.parse_args()

    payload = json.loads(pathlib.Path(args.input_json).read_text())
    items = payload.get("items", [])
    grouped = defaultdict(list)
    aux_rows = []

    for item in items:
      name = item["metadata"]["name"]
      if REP_RE.search(name):
        grouped[base_name(name)].append(item)
      else:
        aux_rows.append({
            "name": name,
            "phase": item.get("status", {}).get("phase", ""),
            "verdict": item.get("status", {}).get("verdict", ""),
            "target": target_label(item),
        })

    rows = []
    output = {"gates": {}, "auxiliary": aux_rows}
    for gate_name, gate_items in sorted(grouped.items()):
        scores = []
        verdicts = []
        sample_sizes = []
        phases = []
        for item in sorted(gate_items, key=lambda it: it["metadata"]["name"]):
            status = item.get("status", {})
            if isinstance(status.get("qualityScore"), (int, float)):
                scores.append(float(status["qualityScore"]))
            verdicts.append(status.get("verdict", ""))
            if isinstance(status.get("samples"), int):
                sample_sizes.append(status["samples"])
            phases.append(status.get("phase", ""))
        verdict_mix = "/".join(verdicts)
        row = {
            "gate": gate_name,
            "target": target_label(gate_items[0]),
            "stability": stability(verdicts),
            "verdict_mix": verdict_mix,
            "mean_score": mean(scores),
            "std_score": stddev(scores),
            "samples": max(sample_sizes) if sample_sizes else 0,
            "repetitions": len(gate_items),
            "phases": phases,
        }
        rows.append(row)
        output["gates"][gate_name] = row

    pathlib.Path(args.output_json).write_text(json.dumps(output, indent=2))

    md_lines = [
        "# E3 Quality Gate Campaign",
        "",
        "- Threshold: `minSamples >= 20`, `5` repetitions per gate.",
        f"- Counted gates: `{len(rows)}`",
        "",
        "| Gate | Target | Stability | Mean score | Std | N | Reps |",
        "|---|---|---|---:|---:|---:|---:|",
    ]
    for row in rows:
        md_lines.append(
            f"| {row['gate']} | {row['target']} | {row['stability']} | {row['mean_score']:.3f} | "
            f"{row['std_score']:.3f} | {row['samples']} | {row['repetitions']} |"
        )
    md_lines.extend([
        "",
        "## Auxiliary Gates",
        "",
        "| Gate | Target | Phase | Verdict |",
        "|---|---|---|---|",
    ])
    for row in aux_rows:
        md_lines.append(f"| {row['name']} | {row['target']} | {row['phase']} | {row['verdict']} |")
    pathlib.Path(args.output_md).write_text("\n".join(md_lines) + "\n")

    pathlib.Path(args.output_tex).write_text(render_tex(rows))


if __name__ == "__main__":
    main()
