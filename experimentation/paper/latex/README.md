# LaTeX Source

This folder is the canonical manuscript source for the revised JNCA-oriented
paper.

```text
latex/
├── main.tex
├── references.bib
└── figures/
```

## Compile

```bash
pdflatex main
bibtex main
pdflatex main
pdflatex main
```

All figures are generated from repository-relative inputs in `../../results`,
`../../results-stats`, and `../../results-bench`.

Provider credentials and secrets are not included in the artifact.
