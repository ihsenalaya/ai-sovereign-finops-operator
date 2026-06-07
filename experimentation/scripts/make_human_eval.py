#!/usr/bin/env python3
"""Build a BLIND human-evaluation package from already-collected answers.

Samples N (prompt, answer) pairs from the objective-benchmark answers, shuffles
them, hides the model identity, and writes:
  human_eval/sheet.csv   — for evaluators to fill (human_score 1-5, human_correct 0/1)
  human_eval/key.csv     — hidden ground truth + judge score (for later agreement)
  human_eval/README.md   — instructions for 2 blind evaluators

No human scores are invented; the sheet ships empty. analyze_human_eval.py
computes human-vs-exact-match and human-vs-judge agreement once it is filled.
"""
import argparse
import csv
import glob
import json
import os
import random


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bench-cache", default="results-bench/cache.json")
    ap.add_argument("--judgetruth", default="results/judgevstruth.csv")
    ap.add_argument("--datasets", default="datasets-public")
    ap.add_argument("--out", default="human_eval")
    ap.add_argument("--n", type=int, default=100)
    ap.add_argument("--seed", type=int, default=42)
    args = ap.parse_args()
    os.makedirs(args.out, exist_ok=True)

    # prompt id -> text/reference
    ptext, pref = {}, {}
    for f in glob.glob(f"{args.datasets}/*.json"):
        d = json.load(open(f))
        for p in d["prompts"]:
            ptext[p["id"]] = p["text"]; pref[p["id"]] = p.get("reference", "")

    answers = json.load(open(args.bench_cache)).get("answers", {})

    # judgevstruth gives objective correctness + judge score per (model, prompt).
    base = []
    if os.path.exists(args.judgetruth):
        for r in csv.DictReader(open(args.judgetruth)):
            key = f"{r['model']}|{r['prompt']}"
            ans = answers.get(key, {}).get("Text", "")
            if ans and r["prompt"] in ptext:
                base.append({"prompt_id": r["prompt"], "model": r["model"], "dataset": r["dataset"],
                             "exact_match": int(r["correct"]), "judge_score": int(r["judge_score"]),
                             "prompt": ptext[r["prompt"]], "answer": ans})
    if not base:
        raise SystemExit("no joinable answers found (run judgevstruth + benchmark first)")

    rng = random.Random(args.seed)
    rng.shuffle(base)
    sample = base[:args.n]

    with open(f"{args.out}/sheet.csv", "w", newline="") as fh:
        w = csv.writer(fh)
        w.writerow(["eval_id", "prompt", "answer", "human_score_1to5", "human_correct_0or1"])
        for i, r in enumerate(sample, 1):
            w.writerow([f"H{i:03d}", r["prompt"], r["answer"], "", ""])
    with open(f"{args.out}/key.csv", "w", newline="") as fh:
        w = csv.writer(fh)
        w.writerow(["eval_id", "dataset", "prompt_id", "model", "exact_match", "judge_score"])
        for i, r in enumerate(sample, 1):
            w.writerow([f"H{i:03d}", r["dataset"], r["prompt_id"], r["model"], r["exact_match"], r["judge_score"]])

    open(f"{args.out}/README.md", "w").write(
        "# Blind human evaluation\n\n"
        f"{len(sample)} (prompt, answer) pairs, model identity hidden, order randomized.\n\n"
        "## For each evaluator (>=2 independent)\n"
        "- Open `sheet.csv`. For every row fill:\n"
        "  - `human_score_1to5`: overall answer quality 1 (useless/incorrect) .. 5 (excellent).\n"
        "  - `human_correct_0or1`: 1 if the answer is factually correct, else 0.\n"
        "- Do NOT look at `key.csv` (it holds the ground truth + LLM-judge scores).\n"
        "- Save as `sheet_<evaluator>.csv`.\n\n"
        "## Analysis\n"
        "`python3 scripts/analyze_human_eval.py --sheets human_eval/sheet_*.csv` computes human accuracy,\n"
        "human-vs-exact-match agreement, and human-vs-LLM-judge Cohen's kappa — to validate (or temper)\n"
        "the LLM-judge as a quality proxy.\n"
    )
    print(f"wrote {args.out}/sheet.csv ({len(sample)} rows), key.csv, README.md — ready for human scoring (empty).")


if __name__ == "__main__":
    main()
