# Inter-judge agreement — judge_gpt-4o vs judge_mistral-large (n=120)

- mean score: judge_gpt-4o=4.608, judge_mistral-large=4.858
- Cohen's quadratic-weighted kappa: **0.403**
- Krippendorff's alpha (ordinal): **0.386**
- Spearman rho: **0.613** (p=1e-13)
- exact agreement: 82.5%  ·  within-1: 94.2%

Interpretation: kappa/alpha > 0.6 = substantial agreement; this supports using the
LLM judge as a quality proxy. Lower values would weaken quality claims (reported honestly).
