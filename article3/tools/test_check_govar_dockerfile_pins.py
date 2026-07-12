#!/usr/bin/env python3
"""Focused regression tests for immutable GOV-AR container bases."""

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("check_govar_dockerfile_pins.py")
DIGEST = "sha256:" + "a" * 64
PINNED = f"registry.example/base:readable@{DIGEST}"


class DockerfilePinCheckTest(unittest.TestCase):
    def run_check(self, from_reference: str) -> subprocess.CompletedProcess[str]:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            dockerfile = root / "Dockerfile"
            provenance = root / "provenance.json"
            dockerfile.write_text(f"FROM {from_reference}\n", encoding="utf-8")
            provenance.write_text(
                json.dumps(
                    {
                        "base_images": [
                            {
                                "pinned_ref": PINNED,
                                "index_digest": DIGEST,
                                "platform": "linux/amd64",
                                "platform_manifest_digest": "sha256:" + "b" * 64,
                            }
                        ]
                    }
                ),
                encoding="utf-8",
            )
            return subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT),
                    "--dockerfile",
                    str(dockerfile),
                    "--provenance",
                    str(provenance),
                ],
                check=False,
                capture_output=True,
                text=True,
            )

    def test_accepts_readable_tag_pinned_to_sha256(self) -> None:
        result = self.run_check(PINNED)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_rejects_mutable_tag(self) -> None:
        result = self.run_check("registry.example/base:readable")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("external FROM must", result.stderr)


if __name__ == "__main__":
    unittest.main()
