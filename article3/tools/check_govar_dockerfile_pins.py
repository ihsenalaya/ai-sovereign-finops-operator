#!/usr/bin/env python3
"""Reject mutable external FROM references in the GOV-AR admission image."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DOCKERFILE = ROOT / "operateur" / "Dockerfile.gov-ar-admission"
PROVENANCE = ROOT / "article3" / "provenance" / "build_base_images.json"
PINNED_REFERENCE = re.compile(
    r"^(?P<tagged>[^\s@]+:[^\s@]+)@(?P<digest>sha256:[0-9a-f]{64})$"
)


def validate(dockerfile: Path, provenance: Path) -> list[str]:
    document = json.loads(provenance.read_text(encoding="utf-8"))
    declared = {
        item["pinned_ref"]: item for item in document.get("base_images", [])
    }
    errors: list[str] = []
    external: list[str] = []

    for number, raw_line in enumerate(
        dockerfile.read_text(encoding="utf-8").splitlines(), start=1
    ):
        line = raw_line.strip()
        if not line.upper().startswith("FROM "):
            continue
        fields = line.split()
        try:
            image = fields[1] if not fields[1].startswith("--platform=") else fields[2]
        except IndexError:
            errors.append(f"line {number}: malformed FROM instruction")
            continue
        if image == "scratch":
            continue
        match = PINNED_REFERENCE.fullmatch(image)
        if not match:
            errors.append(
                f"line {number}: external FROM must retain a readable tag and pin a "
                f"sha256 digest: {image}"
            )
            continue
        external.append(image)
        evidence = declared.get(image)
        if evidence is None:
            errors.append(f"line {number}: pin is absent from {provenance}")
            continue
        if evidence.get("index_digest") != match.group("digest"):
            errors.append(f"line {number}: provenance index digest does not match FROM")
        if evidence.get("platform") != "linux/amd64":
            errors.append(f"line {number}: linux/amd64 manifest verification is absent")
        if not re.fullmatch(
            r"sha256:[0-9a-f]{64}", evidence.get("platform_manifest_digest", "")
        ):
            errors.append(f"line {number}: invalid linux/amd64 platform digest")

    if set(external) != set(declared):
        errors.append("Dockerfile external bases and provenance entries are not one-to-one")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dockerfile", type=Path, default=DOCKERFILE)
    parser.add_argument("--provenance", type=Path, default=PROVENANCE)
    args = parser.parse_args()
    errors = validate(args.dockerfile, args.provenance)
    if errors:
        for error in errors:
            print(f"FAIL: {error}", file=sys.stderr)
        return 1
    print("PASS: all external FROM references are immutable and evidenced")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
