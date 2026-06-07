# Statistical summary — N=30 repetitions (temperature>0)

Per-rep aggregates (cost=sum, quality/latency=mean). Tests are vs B1-premium-static on
the per-rep distributions (Mann-Whitney U, Cliff's delta, Cohen's d).


## Cost (EUR, total/rep)

| strategy | mean | 95% CI | vs premium p (MWU) | Cliff δ | Cohen d |
|---|---:|---|---:|---:|---:|
| B1-premium-static | 0.0385 | [0.0382, 0.0389] | nan | +nan | +nan |
| B2-round-robin | 0.0141 | [0.0140, 0.0143] | 3.02e-11 | -1.000 | -37.236 |
| B3-least-cost | 0.0007 | [0.0007, 0.0007] | 3e-11 | -1.000 | -61.748 |
| B4-static-policy | 0.0208 | [0.0206, 0.0209] | 3.02e-11 | -1.000 | -25.853 |
| B5-budget-hard-block | 0.0380 | [0.0376, 0.0383] | 0.0378 | -0.313 | -0.613 |
| B6-ours | 0.0110 | [0.0109, 0.0110] | 3.02e-11 | -1.000 | -43.444 |

## Quality (norm, mean/rep)

| strategy | mean | 95% CI | vs premium p (MWU) | Cliff δ | Cohen d |
|---|---:|---|---:|---:|---:|
| B1-premium-static | 0.9362 | [0.9336, 0.9389] | nan | +nan | +nan |
| B2-round-robin | 0.9238 | [0.9212, 0.9264] | 2.96e-09 | -0.884 | -1.792 |
| B3-least-cost | 0.8694 | [0.8687, 0.8701] | 2.36e-12 | -1.000 | -12.966 |
| B4-static-policy | 0.9500 | [0.9456, 0.9544] | 4.4e-06 | +0.682 | +1.412 |
| B5-budget-hard-block | 0.9354 | [0.9325, 0.9383] | 0.848 | -0.029 | -0.113 |
| B6-ours | 0.9587 | [0.9564, 0.9611] | 5.57e-11 | +0.973 | +3.375 |

## Latency (ms, mean/rep)

| strategy | mean | 95% CI | vs premium p (MWU) | Cliff δ | Cohen d |
|---|---:|---|---:|---:|---:|
| B1-premium-static | 997.6542 | [942.6296, 1052.6787] | nan | +nan | +nan |
| B2-round-robin | 1078.6217 | [1048.5010, 1108.7423] | 3.83e-05 | +0.620 | +0.682 |
| B3-least-cost | 707.2083 | [699.5500, 714.8667] | 3.02e-11 | -1.000 | -2.761 |
| B4-static-policy | 857.0475 | [834.5463, 879.5487] | 7.77e-09 | -0.869 | -1.249 |
| B5-budget-hard-block | 984.2033 | [947.3419, 1021.0648] | 0.751 | -0.049 | -0.107 |
| B6-ours | 1506.2358 | [1459.1338, 1553.3379] | 4.62e-10 | +0.938 | +3.708 |
