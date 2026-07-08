#!/usr/bin/env python3
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DATASET_DIR = ROOT / "datasets" / "golden"
TEMPLATE_PATH = DATASET_DIR / "configmaps.yaml"


def indent_block(text: str) -> str:
    return "\n".join(f"    {line}" if line else "    " for line in text.rstrip().splitlines()) + "\n"


def main() -> None:
    template = TEMPLATE_PATH.read_text()
    replacements = {
        "{{FINANCE_PROMPTS}}": indent_block((DATASET_DIR / "finance-quality-golden.prompts.yaml").read_text()),
        "{{LEGAL_PROMPTS}}": indent_block((DATASET_DIR / "legal-quality-golden.prompts.yaml").read_text()),
        "{{MARKETING_PROMPTS}}": indent_block((DATASET_DIR / "marketing-quality-golden.prompts.yaml").read_text()),
        "{{RH_PROMPTS}}": indent_block((DATASET_DIR / "rh-quality-golden.prompts.yaml").read_text()),
    }
    rendered = template
    for needle, value in replacements.items():
        rendered = rendered.replace(needle, value.rstrip("\n"))
    print(rendered, end="")


if __name__ == "__main__":
    main()
