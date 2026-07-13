#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import json
import os
import stat
import subprocess
import tempfile
import unittest
from pathlib import Path


HERE = Path(__file__).resolve().parent
REPO = HERE.parents[2]
PROVENANCE = REPO / "article3/provenance/kind_infrastructure.json"


class SourceTests(unittest.TestCase):
    def test_shell_syntax(self) -> None:
        for script in sorted(HERE.glob("*.sh")):
            subprocess.run(["bash", "-n", str(script)], check=True)

    def test_exact_profiles(self) -> None:
        data = json.loads(PROVENANCE.read_text(encoding="utf-8"))
        self.assertEqual(
            data["profiles"],
            {
                "dev": {"cluster": "article3-dev", "nodes": 2, "cni": "kindnet"},
                "validation": {"cluster": "article3-validation", "nodes": 3, "cni": "calico"},
                "performance": {
                    "cluster": "article3-performance",
                    "nodes": 4,
                    "cni": "calico",
                    "minimum_available_memory_plus_swap_kib": 2097152,
                },
            },
        )
        for profile, expected_nodes in (("dev", 2), ("validation", 3), ("performance", 4)):
            source = (HERE / f"cluster-{profile}.yaml").read_text()
            self.assertNotIn("name:", source)
            self.assertEqual(source.count("- role:"), expected_nodes)
            if profile != "dev":
                self.assertIn("disableDefaultCNI: true", source)
                self.assertIn("podSubnet: 192.168.0.0/16", source)

    def test_pins_match_common(self) -> None:
        common = (HERE / "common.sh").read_text(encoding="utf-8")
        data = json.loads(PROVENANCE.read_text(encoding="utf-8"))
        pins = [
            data["kind"]["node_image"],
            data["calico"]["manifest_sha256"],
            data["calico"]["digest_pinned_manifest_sha256"],
            *data["calico"]["images"].values(),
        ]
        for pin in pins:
            self.assertIn(pin, common)
        self.assertRegex(data["retrieved_at_utc"], r"^2026-07-12T\d{2}:\d{2}:\d{2}Z$")
        create = (HERE / "create.sh").read_text(encoding="utf-8")
        self.assertIn('crictl pull "${image}"', create)
        self.assertIn('crictl inspecti "${image}"', create)
        self.assertIn('any("/import-" in item for item in digests)', create)
        self.assertIn("retaining ownership record", create)
        self.assertLess(create.index('ownership.py" create'), create.index('if [[ "${cni}" == calico ]]'))
        self.assertNotIn("kind load docker-image", create)
        self.assertNotIn("ctr --namespace=k8s.io images import", create)

    def test_provenance_schema(self) -> None:
        data = json.loads(PROVENANCE.read_text(encoding="utf-8"))
        self.assertEqual(
            set(data),
            {
                "schema_version",
                "retrieved_at_utc",
                "kind",
                "calico",
                "network_policy_probe",
                "profiles",
            },
        )
        self.assertEqual(data["schema_version"], 1)
        self.assertRegex(data["calico"]["release_tag_commit"], r"^[0-9a-f]{40}$")
        self.assertIsInstance(data["calico"]["manifest_bytes"], int)
        self.assertGreater(data["calico"]["manifest_bytes"], 0)
        digests = [
            data["calico"]["manifest_sha256"],
            data["calico"]["digest_pinned_manifest_sha256"],
            *data["calico"]["images"].values(),
        ]
        for digest in digests:
            self.assertRegex(digest, r"^(?:sha256:)?[0-9a-f]{64}$")
        self.assertRegex(
            data["kind"]["node_image"],
            r"^kindest/node:v1\.35\.0@sha256:[0-9a-f]{64}$",
        )
        self.assertRegex(
            data["network_policy_probe"]["image"],
            r"^docker\.io/library/busybox@sha256:[0-9a-f]{64}$",
        )
        self.assertEqual(
            data["network_policy_probe"]["expected_sequence"],
            ["allow_before_policy", "deny_with_empty_egress", "allow_after_delete"],
        )

    def test_diagnostics_are_infrastructure_scoped(self) -> None:
        source = (HERE / "collect-diagnostics.sh").read_text(encoding="utf-8")
        self.assertIn("--namespaces='kube-system'", source)
        self.assertIn("kubectl -n kube-system get pods", source)
        self.assertNotIn("kubectl get pods -A", source)
        self.assertNotRegex(source, r"kubectl(?:\s+get)?[^\n]*\s-A(?:\s|$)")

    def test_no_legacy_cluster_alias_in_active_scripts(self) -> None:
        for script in HERE.glob("*.sh"):
            source = script.read_text(encoding="utf-8")
            self.assertNotRegex(source, r"CLUSTER_NAME[^\n]*(gov-ar|greenops)", str(script))
        self.assertFalse((HERE / "cluster.yaml").exists())

    def test_application_deployment_is_closed(self) -> None:
        result = subprocess.run(
            ["bash", str(HERE / "deploy_experiments.sh")], text=True, capture_output=True
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("refusing experiment deployment", result.stderr)


class DestructiveNegativeTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory(prefix="article3-kind-negative-")
        root = Path(self.temp.name)
        self.bin = root / "bin"
        self.bin.mkdir()
        self.state = root / "state"
        self.log = root / "calls.log"
        self.env = os.environ.copy()
        self.env.update(
            {
                "PATH": f"{self.bin}:{self.env['PATH']}",
                "ARTICLE3_KIND_STATE_DIR": str(self.state),
                "FAKE_CALL_LOG": str(self.log),
                "FAKE_CLUSTERS": "",
                "FAKE_NODE_IMAGE": json.loads(PROVENANCE.read_text())["kind"]["node_image"],
            }
        )
        self._fake(
            "docker",
            """
if [[ "${1:-}" == inspect ]]; then
  printf '[{"Id":"fake-id","Config":{"Labels":{"io.x-k8s.kind.cluster":"%s"},"Image":"%s"}}]\\n' \\
    "${FAKE_CLUSTER_NAME:-article3-validation}" "${FAKE_NODE_IMAGE}"
fi
exit 0
""",
        )
        self._fake(
            "kind",
            """
printf '%s\\n' "$*" >>"$FAKE_CALL_LOG"
if [[ "$1 $2" == "get clusters" ]]; then
  if [[ -n "${FAKE_CLUSTER_STATE_FILE:-}" ]]; then cat "$FAKE_CLUSTER_STATE_FILE"; else printf '%s\\n' "$FAKE_CLUSTERS"; fi
  exit 0
fi
if [[ "$1 $2" == "get nodes" ]]; then printf '%s\\n' fake-node; exit 0; fi
if [[ "$1 $2" == "delete cluster" && "${FAKE_DELETE_REMOVES_CLUSTER:-}" == 1 ]]; then
  : >"$FAKE_CLUSTER_STATE_FILE"
fi
exit 0
""",
        )

    def tearDown(self) -> None:
        self.temp.cleanup()

    def _fake(self, name: str, body: str) -> None:
        path = self.bin / name
        path.write_text(f"#!/usr/bin/env bash\nset -euo pipefail\n{body}\n", encoding="utf-8")
        path.chmod(path.stat().st_mode | stat.S_IXUSR)

    def run_script(self, name: str, **extra: str) -> subprocess.CompletedProcess[str]:
        env = self.env | extra
        return subprocess.run(
            ["bash", str(HERE / name)], env=env, text=True, capture_output=True
        )

    def calls(self) -> str:
        return self.log.read_text(encoding="utf-8") if self.log.exists() else ""

    def write_dev_record(self) -> Path:
        self.state.mkdir(exist_ok=True)
        image = self.env["FAKE_NODE_IMAGE"]
        config = HERE / "cluster-dev.yaml"
        record = {
            "schema_version": 2,
            "owner": "article3-q1-recovery",
            "cluster": "article3-dev",
            "profile": "dev",
            "config_sha256": hashlib.sha256(config.read_bytes()).hexdigest(),
            "node_image": image,
            "cni": "kindnet",
            "calico_version": None,
            "calico_pinned_manifest_sha256": None,
            "nodes": [
                {
                    "name": "fake-node",
                    "container_id": "fake-id",
                    "cluster_label": "article3-dev",
                    "configured_image": image,
                }
            ],
        }
        path = self.state / "article3-dev.json"
        path.write_text(json.dumps(record), encoding="utf-8")
        path.chmod(0o600)
        return path

    def test_rejects_unrelated_name_before_delete(self) -> None:
        result = self.run_script(
            "destroy.sh", PROFILE="validation", CLUSTER_NAME="gov-ar", FAKE_CLUSTERS="gov-ar"
        )
        self.assertEqual(result.returncode, 2)
        self.assertNotIn("delete cluster", self.calls())

    def test_rejects_exact_but_unowned_existing_cluster(self) -> None:
        result = self.run_script(
            "destroy.sh", PROFILE="validation", FAKE_CLUSTERS="article3-validation"
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("unowned", result.stderr)
        self.assertNotIn("delete cluster", self.calls())

    def test_rejects_symlinked_ownership_record(self) -> None:
        self.state.mkdir()
        target = Path(self.temp.name) / "target.json"
        target.write_text("{}", encoding="utf-8")
        (self.state / "article3-validation.json").symlink_to(target)
        result = self.run_script(
            "destroy.sh", PROFILE="validation", FAKE_CLUSTERS="article3-validation"
        )
        self.assertEqual(result.returncode, 2)
        self.assertNotIn("delete cluster", self.calls())

    def test_rejects_permissive_ownership_record(self) -> None:
        self.state.mkdir()
        record = self.state / "article3-validation.json"
        record.write_text("{}\n", encoding="utf-8")
        record.chmod(0o644)
        result = self.run_script(
            "destroy.sh", PROFILE="validation", FAKE_CLUSTERS="article3-validation"
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("0600", result.stderr)
        self.assertNotIn("delete cluster", self.calls())

    def test_rejects_cni_binding_mismatch(self) -> None:
        self.state.mkdir()
        image = self.env["FAKE_NODE_IMAGE"]
        config = HERE / "cluster-validation.yaml"
        record = {
            "schema_version": 2,
            "owner": "article3-q1-recovery",
            "cluster": "article3-validation",
            "profile": "validation",
            "config_sha256": hashlib.sha256(config.read_bytes()).hexdigest(),
            "node_image": image,
            "cni": "kindnet",
            "calico_version": None,
            "calico_pinned_manifest_sha256": None,
            "nodes": [
                {
                    "name": "fake-node",
                    "container_id": "fake-id",
                    "cluster_label": "article3-validation",
                    "configured_image": image,
                }
            ],
        }
        path = self.state / "article3-validation.json"
        path.write_text(json.dumps(record), encoding="utf-8")
        path.chmod(0o600)
        result = self.run_script(
            "destroy.sh",
            PROFILE="validation",
            FAKE_CLUSTERS="article3-validation",
            FAKE_CLUSTER_NAME="article3-validation",
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("ownership verification failed", result.stderr)
        self.assertNotIn("delete cluster", self.calls())

    def test_rejects_non_posix_state_storage(self) -> None:
        fake_stat = self.bin / "stat"
        fake_stat.write_text(
            "#!/usr/bin/env bash\nprintf '777\\n'\n", encoding="utf-8"
        )
        fake_stat.chmod(fake_stat.stat().st_mode | stat.S_IXUSR)
        result = self.run_script("destroy.sh", PROFILE="dev")
        self.assertEqual(result.returncode, 2)
        self.assertIn("does not preserve POSIX mode 0600", result.stderr)
        self.assertNotIn("delete cluster", self.calls())

    def test_absent_cluster_destroy_is_idempotent(self) -> None:
        result = self.run_script("destroy.sh", PROFILE="dev")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("already absent", result.stdout)
        self.assertNotIn("delete cluster", self.calls())

    def test_destroy_retains_record_until_absence_is_verified(self) -> None:
        record = self.write_dev_record()
        result = self.run_script(
            "destroy.sh",
            PROFILE="dev",
            FAKE_CLUSTERS="article3-dev",
            FAKE_CLUSTER_NAME="article3-dev",
        )
        self.assertEqual(result.returncode, 2)
        self.assertTrue(record.exists())
        self.assertIn("retaining ownership record", result.stderr)
        self.assertIn("delete cluster", self.calls())

    def test_destroy_removes_record_after_absence_is_verified(self) -> None:
        record = self.write_dev_record()
        cluster_state = Path(self.temp.name) / "clusters.txt"
        cluster_state.write_text("article3-dev\n", encoding="utf-8")
        result = self.run_script(
            "destroy.sh",
            PROFILE="dev",
            FAKE_CLUSTER_NAME="article3-dev",
            FAKE_CLUSTER_STATE_FILE=str(cluster_state),
            FAKE_DELETE_REMOVES_CLUSTER="1",
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(record.exists())
        self.assertIn("destroyed owned cluster", result.stdout)

    def test_create_does_not_adopt_existing_cluster(self) -> None:
        result = self.run_script(
            "create.sh", PROFILE="performance", FAKE_CLUSTERS="article3-performance"
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("unowned", result.stderr)
        self.assertNotIn("create cluster", self.calls())

    def test_performance_capacity_preflight_fails_before_create(self) -> None:
        result = self.run_script(
            "create.sh",
            PROFILE="performance",
            ARTICLE3_KIND_AVAILABLE_KIB_OVERRIDE="1024",
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("below 2097152 KiB safety floor", result.stderr)
        self.assertNotIn("create cluster", self.calls())


if __name__ == "__main__":
    unittest.main(verbosity=2)
