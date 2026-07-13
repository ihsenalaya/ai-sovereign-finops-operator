#!/usr/bin/env python3
"""Create and verify fail-closed local ownership records for Article 3 Kind clusters."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import stat
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


def output(*args: str) -> str:
    return subprocess.check_output(list(args), text=True).strip()


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def actual_nodes(cluster: str) -> list[dict[str, str]]:
    names = sorted(output("kind", "get", "nodes", "--name", cluster).splitlines())
    nodes: list[dict[str, str]] = []
    for name in names:
        values = json.loads(output("docker", "inspect", name))[0]
        nodes.append(
            {
                "name": name,
                "container_id": values["Id"],
                "cluster_label": values.get("Config", {}).get("Labels", {}).get(
                    "io.x-k8s.kind.cluster", ""
                ),
                "configured_image": values.get("Config", {}).get("Image", ""),
            }
        )
    return nodes


def validate_args(cluster: str, profile: str) -> None:
    if profile not in {"dev", "validation", "performance"}:
        raise ValueError(f"invalid profile {profile}")
    expected = f"article3-{profile}"
    if cluster != expected:
        raise ValueError(f"cluster/profile mismatch: expected {expected}, got {cluster}")


def create(args: argparse.Namespace) -> None:
    validate_args(args.cluster, args.profile)
    state = Path(args.state)
    if state.exists() or state.is_symlink():
        raise FileExistsError(f"state already exists: {state}")
    nodes = actual_nodes(args.cluster)
    if not nodes or any(node["cluster_label"] != args.cluster for node in nodes):
        raise ValueError("Kind node labels do not bind every node to the requested cluster")
    if any(node["configured_image"] != args.node_image for node in nodes):
        raise ValueError("actual node image does not match the pinned node image")
    payload = {
        "schema_version": 2,
        "owner": "article3-q1-recovery",
        "cluster": args.cluster,
        "profile": args.profile,
        "config_sha256": sha256(Path(args.config)),
        "node_image": args.node_image,
        "cni": args.cni,
        "calico_version": args.calico_version if args.cni == "calico" else None,
        "calico_pinned_manifest_sha256": args.calico_sha256 if args.cni == "calico" else None,
        "created_at_utc": datetime.now(timezone.utc).isoformat(),
        "nodes": nodes,
    }
    state.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(state, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, indent=2, sort_keys=True)
        handle.write("\n")


def verify(args: argparse.Namespace) -> None:
    validate_args(args.cluster, args.profile)
    state = Path(args.state)
    mode = state.lstat().st_mode
    if stat.S_ISLNK(mode) or not stat.S_ISREG(mode):
        raise ValueError("ownership state must be a regular file, never a symlink")
    if mode & 0o077:
        raise ValueError("ownership state permissions must be 0600 or stricter")
    data = json.loads(state.read_text(encoding="utf-8"))
    expected = {
        "schema_version": 2,
        "owner": "article3-q1-recovery",
        "cluster": args.cluster,
        "profile": args.profile,
        "config_sha256": sha256(Path(args.config)),
        "node_image": args.node_image,
        "cni": args.cni,
        "calico_version": args.calico_version if args.cni == "calico" else None,
        "calico_pinned_manifest_sha256": (
            args.calico_sha256 if args.cni == "calico" else None
        ),
    }
    mismatches = [key for key, value in expected.items() if data.get(key) != value]
    actual = actual_nodes(args.cluster)
    if data.get("nodes") != actual:
        mismatches.append("nodes")
    if any(node["cluster_label"] != args.cluster for node in actual):
        mismatches.append("cluster_label")
    if any(node["configured_image"] != args.node_image for node in actual):
        mismatches.append("configured_image")
    if mismatches:
        raise ValueError("ownership verification failed: " + ", ".join(sorted(set(mismatches))))
    print(f"ownership verified: {args.cluster} ({len(actual)} nodes)")


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser()
    result.add_argument("action", choices=("create", "verify"))
    result.add_argument("--state", required=True)
    result.add_argument("--cluster", required=True)
    result.add_argument("--profile", required=True)
    result.add_argument("--node-image", required=True)
    result.add_argument("--config", required=True)
    result.add_argument("--cni", default="")
    result.add_argument("--calico-version", default="")
    result.add_argument("--calico-sha256", default="")
    return result


def main() -> int:
    args = parser().parse_args()
    try:
        if args.action == "create":
            create(args)
        else:
            verify(args)
    except (OSError, subprocess.CalledProcessError, ValueError, json.JSONDecodeError) as exc:
        print(f"ownership error: {exc}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
