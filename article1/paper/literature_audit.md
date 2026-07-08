# Literature Audit — Article 1

**Purpose.** Traceability record for `references_verified.bib`. Every entry is a
REAL, verifiable work. Author lists for arXiv entries were pulled from the arXiv
API on 2026-07-04; peer-reviewed venues were confirmed via ACM DL / USENIX /
IEEE / RFC Editor / dblp. **No DOI or citation was invented.** Where only a
preprint was confirmed, the entry is marked as arXiv (with the peer-reviewed
venue in a note only when that venue was independently confirmed).

**Count:** 47 references (≥ 45 required), across 9 themes.

## Verification status legend
- `VENUE+DOI` — peer-reviewed venue and DOI both confirmed.
- `VENUE` — peer-reviewed venue confirmed; no DOI asserted.
- `ARXIV` — arXiv preprint confirmed (id + authors verified via arXiv API).
- `ARXIV+VENUE` — arXiv confirmed AND a peer-reviewed venue independently seen.
- `WEB` — official project/blog page (non-peer-reviewed, cited as such).

## A. TEE hardware foundations & confidential VMs
| key | status | anchor |
|-----|--------|--------|
| cheng2023tdx | VENUE+DOI / ARXIV | ACM CSUR 10.1145/3652597; arXiv:2303.15540 |
| paradzik2024sevsnp | ARXIV | arXiv:2403.10296 |
| misono2024cvms | VENUE+DOI | POMACS 8(3), 10.1145/3700418; dblp MisonoSSB24 |
| costan2016sgx | VENUE | IACR ePrint 2016/086 |
| lee2020keystone | VENUE+DOI | EuroSys 2020, 10.1145/3342195.3387532 |
| xu2023virtcca | ARXIV | arXiv:2306.11011 |
| cerdeira2022rezone | ARXIV+VENUE | arXiv:2203.01025; USENIX Security 2022 |
| michaud2025teesok | ARXIV | arXiv:2512.22090 |

## B. Remote attestation: architecture, protocols, trusted channels
| key | status | anchor |
|-----|--------|--------|
| birkholz2023rats | VENUE+DOI | RFC 9334, 10.17487/RFC9334 |
| chen2019opera | VENUE+DOI | CCS 2019, 10.1145/3319535.3354220 |
| knauth2018ratls | ARXIV | arXiv:1801.05863 (RA-TLS) |
| akama2024raweb | ARXIV | arXiv:2411.01340 |
| liu2020confidattest | ARXIV | arXiv:2007.10513 |
| antonino2023flexattest | ARXIV | arXiv:2305.09351 |

## C. TEE attacks (threat-model motivation)
| key | status | anchor |
|-----|--------|--------|
| li2021cipherleaks | VENUE | USENIX Security 2021 |
| liu2025teecontainers | ARXIV | arXiv:2508.20962 |
| zhang2023noprivacy | ARXIV+VENUE | arXiv:2310.07152; IEEE S&P 2024 |

## D. TEE systems: library OS, secure containers
| key | status | anchor |
|-----|--------|--------|
| arnautov2016scone | VENUE | OSDI 2016 |
| tsai2017graphene | VENUE+DOI | USENIX ATC 2017, 10.5555/3154690.3154752 |
| shen2020occlum | VENUE | ASPLOS 2020 |
| sarkar2024hastee | ARXIV | arXiv:2401.08901 |
| hartono2024crisp | ARXIV | arXiv:2408.06822 |

## E. Confidential containers & confidential Kubernetes
| key | status | anchor |
|-----|--------|--------|
| yang2026dstack | ARXIV | arXiv:2606.03323 (pod-level attestation, dstack) |
| k8s2023confidential | WEB | kubernetes.io blog 2023 |
| coco2024 | WEB | confidentialcontainers.org (CNCF) |

## F. Confidential ML / AI inference on TEEs
| key | status | anchor |
|-----|--------|--------|
| tramer2019slalom | ARXIV+VENUE | arXiv:1806.03287; ICLR 2019 |
| mo2020darknetz | VENUE+DOI | MobiSys 2020, 10.1145/3386901.3388946 |
| sun2020shadownet | ARXIV+VENUE | arXiv:2011.05905; IEEE S&P 2023 |
| li2024teeslice | ARXIV | arXiv:2411.09945 |
| hua2020guardnn | ARXIV+VENUE | arXiv:2008.11632; DAC 2022 |
| hashemi2022darknight | ARXIV+VENUE | arXiv:2207.00083; MICRO 2021 |
| ma2020s3ml | ARXIV | arXiv:2010.06212 |
| lee2020ppml | ARXIV | arXiv:2009.04390 |
| hu2024sesemi | ARXIV | arXiv:2412.11640 |
| schambach2026enclavex | ARXIV | arXiv:2606.31408 |
| yu2025dualprivacy | ARXIV | arXiv:2509.09091 |
| abdollahi2025ondevice | ARXIV+VENUE | arXiv:2504.08508; SysTEX 2025 |
| abdollahi2026agentee | ARXIV | arXiv:2604.18231 |

## G. Systematization of knowledge — ML + confidential computing
| key | status | anchor |
|-----|--------|--------|
| mo2022mlccsok | ARXIV+VENUE | arXiv:2208.10134; ACM CSUR |
| duy2021cmlsok | ARXIV+VENUE | arXiv:2111.03308; IEEE Access |

## H. GPU trusted execution & confidential GPUs (future-work positioning)
| key | status | anchor |
|-----|--------|--------|
| volos2018graviton | VENUE+DOI | OSDI 2018, 10.5555/3291168.3291219 |
| hunt2020telekine | VENUE | NSDI 2020 |
| zhu2024h100bench | ARXIV | arXiv:2409.03992 |
| gu2025gpucc | ARXIV | arXiv:2507.02770 |

## I. Orchestration security, supply chain, verifiable ML provenance
| key | status | anchor |
|-----|--------|--------|
| duddu2024laminator | ARXIV+VENUE | arXiv:2406.17548; ACM CODASPY 2025 |
| ali2024cmsweb | ARXIV | arXiv:2405.15342 |
| enyedi2026modrepos | ARXIV | arXiv:2603.02512 |

## Gaps / caveats for the writing pass
- For entries tagged `ARXIV+VENUE`, cite the peer-reviewed venue in the final
  text only if the camera-ready is confirmed; otherwise keep the arXiv handle.
- `WEB` entries (Confidential Kubernetes blog, CoCo project) are cited as
  engineering references, never as peer-reviewed evidence.
- Two named systems in the roadmap — **C8s** and **dstack-capsule** — are
  discussed in related-work; dstack is covered by yang2026dstack. C8s is an
  industrial/architecture effort; it is positioned qualitatively and must NOT be
  given a fabricated citation. If a peer-reviewed C8s paper is later confirmed,
  add it; until then it is referenced descriptively.
- DOIs are asserted ONLY where seen during collection. Missing DOIs are left
  out deliberately rather than guessed.
