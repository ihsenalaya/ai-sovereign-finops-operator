#!/usr/bin/env python3
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TOOL = ROOT / "article3" / "tools" / "verify_govar_audit.py"
FIXTURE = ROOT / "article3" / "tests" / "fixtures" / "govar_audit_valid.json"
CHECKPOINT = ROOT / "article3" / "tests" / "fixtures" / "govar_audit_checkpoint.json"


class AuditVerifierTest(unittest.TestCase):
    def test_valid_fixture_and_tamper(self):
        valid = subprocess.run([sys.executable, str(TOOL), str(FIXTURE), "--checkpoint", str(CHECKPOINT)], check=False,
                               text=True, capture_output=True)
        self.assertEqual(valid.returncode, 0, valid.stderr + valid.stdout)
        result = json.loads(valid.stdout)
        self.assertTrue(result["verified"])
        self.assertEqual(result["tenant_count"], 1)
        self.assertEqual(result["event_count"], 2)
        self.assertRegex(result["result_sha256"], r"^[0-9a-f]{64}$")

        rows = json.loads(FIXTURE.read_text(encoding="utf-8"))
        tampered_rows = [dict(row) for row in rows]
        tampered_rows[1]["reason"] = "tampered"
        with tempfile.TemporaryDirectory() as directory:
            tampered = Path(directory) / "tampered.json"
            tampered.write_text(json.dumps(tampered_rows), encoding="utf-8")
            invalid = subprocess.run([sys.executable, str(TOOL), str(tampered)], check=False,
                                     text=True, capture_output=True)
        self.assertEqual(invalid.returncode, 1)
        self.assertFalse(json.loads(invalid.stdout)["verified"])

        for modified in (list(reversed(rows)), rows[:1], []):
            with tempfile.TemporaryDirectory() as directory:
                candidate = Path(directory) / "invalid.json"
                candidate.write_text(json.dumps(modified), encoding="utf-8")
                invalid = subprocess.run(
                    [sys.executable, str(TOOL), str(candidate), "--checkpoint", str(CHECKPOINT)],
                    text=True, capture_output=True, check=False)
            self.assertEqual(invalid.returncode, 1)
            self.assertFalse(json.loads(invalid.stdout)["verified"])

        wrong = {"event_count": 2, "chain_heads": {"tenant-public-fixture": "0" * 64}}
        with tempfile.TemporaryDirectory() as directory:
            checkpoint = Path(directory) / "wrong-checkpoint.json"
            checkpoint.write_text(json.dumps(wrong), encoding="utf-8")
            invalid = subprocess.run(
                [sys.executable, str(TOOL), str(FIXTURE), "--checkpoint", str(checkpoint)],
                text=True, capture_output=True, check=False)
        self.assertEqual(invalid.returncode, 1)


if __name__ == "__main__":
    unittest.main()
