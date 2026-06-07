# Blind human evaluation

100 (prompt, answer) pairs, model identity hidden, order randomized.

## For each evaluator (>=2 independent)
- Open `sheet.csv`. For every row fill:
  - `human_score_1to5`: overall answer quality 1 (useless/incorrect) .. 5 (excellent).
  - `human_correct_0or1`: 1 if the answer is factually correct, else 0.
- Do NOT look at `key.csv` (it holds the ground truth + LLM-judge scores).
- Save as `sheet_<evaluator>.csv`.

## Analysis
`python3 scripts/analyze_human_eval.py --sheets human_eval/sheet_*.csv` computes human accuracy,
human-vs-exact-match agreement, and human-vs-LLM-judge Cohen's kappa — to validate (or temper)
the LLM-judge as a quality proxy.
