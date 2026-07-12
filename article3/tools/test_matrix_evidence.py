#!/usr/bin/env python3
"""Run Phase-D checks with redacted logs and compile fail-closed evidence."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time

REQUIRED = [
    "format", "go_test", "go_race_govar", "go_vet", "lint",
    "unit_reservations", "transactions", "idempotence", "envtest",
    "postgres_integration", "gateway_e2e", "helm_lint", "helm_install",
    "helm_upgrade", "helm_rollback", "helm_uninstall", "sbom",
    "vulnerability_scan", "secret_scan",
]


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def safe_run(args: list[str], cwd: Path) -> tuple[int, str]:
    try:
        proc = subprocess.run(args, cwd=cwd, stdout=subprocess.PIPE,
                              stderr=subprocess.STDOUT, text=True, timeout=20,
                              check=False)
        return proc.returncode, proc.stdout.strip()
    except (OSError, subprocess.TimeoutExpired) as exc:
        return 127, f"unavailable: {type(exc).__name__}"


def selected_files(root: Path) -> list[str]:
    cmd = ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--",
           "AGENTS.md", "operateur", "article3"]
    proc = subprocess.run(cmd, cwd=root, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
    if proc.returncode != 0:
        raise RuntimeError(proc.stderr.decode("utf-8", "replace"))
    ignored_prefixes = (
        "article3/archive/", "article3/reports/", "operateur/bin/",
        "operateur/cover.out", "operateur/dist/",
    )
    names = []
    for raw in proc.stdout.split(b"\0"):
        if not raw:
            continue
        name = raw.decode("utf-8", "surrogateescape")
        path = root / name
        if name == "propmt.txt" or name.startswith(ignored_prefixes) or not path.is_file():
            continue
        names.append(name)
    return sorted(set(names))


def fingerprint(root: Path) -> dict:
    files = selected_files(root)
    aggregate = hashlib.sha256()
    manifest = []
    for name in files:
        digest = sha256_file(root / name)
        aggregate.update(name.encode("utf-8", "surrogateescape") + b"\0" + digest.encode() + b"\n")
        manifest.append({"path": name, "sha256": digest})
    _, head = safe_run(["git", "rev-parse", "HEAD"], root)
    _, tree = safe_run(["git", "rev-parse", "HEAD^{tree}"], root)
    _, branch = safe_run(["git", "branch", "--show-current"], root)
    diff = subprocess.run(["git", "diff", "--binary", "HEAD", "--", "AGENTS.md", "operateur", "article3"],
                          cwd=root, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=False)
    return {
        "source_sha256": aggregate.hexdigest(),
        "source_file_count": len(files),
        "source_manifest": manifest,
        "git_head": head,
        "git_head_tree": tree,
        "git_branch": branch,
        "git_diff_sha256": hashlib.sha256(diff.stdout).hexdigest(),
    }


def tool_versions(root: Path) -> dict[str, str]:
    commands = {
        "git": ["git", "--version"], "go": ["go", "version"],
        "docker": ["docker", "version", "--format", "{{.Client.Version}}"],
        "kind": ["kind", "version"], "kubectl": ["kubectl", "version", "--client", "--output=yaml"],
        "helm": ["helm", "version", "--short"], "trivy": ["trivy", "--version"],
    }
    return {name: safe_run(command, root)[1][:2000] for name, command in commands.items()}


def scrub_environment() -> tuple[dict[str, str], list[str]]:
    env = dict(os.environ)
    secret_name = re.compile(r"(?i)(token|secret|password|passwd|api.?key|connection.?string|credential)")
    literals = []
    for key in list(env):
        if secret_name.search(key):
            value = env.pop(key)
            if len(value) >= 8:
                literals.append(value)
    return env, sorted(literals, key=len, reverse=True)


def redact(text: str, literals: list[str]) -> str:
    for value in literals:
        text = text.replace(value, "[REDACTED_ENV]")
    patterns = [
        (r"(?i)\b(bearer\s+)[A-Za-z0-9._~+/=-]{12,}", r"\1[REDACTED]"),
        (r"\bgh[opusr]_[A-Za-z0-9]{20,}\b", "[REDACTED_GITHUB_TOKEN]"),
        (r"\bsk-[A-Za-z0-9_-]{20,}\b", "[REDACTED_API_KEY]"),
        (r"(?i)(postgres(?:ql)?://)[^\s/@:]+:[^\s/@]+@", r"\1[REDACTED]@"),
        (r"(?i)https://[A-Za-z0-9.-]+\.(?:openai\.azure\.com|cognitiveservices\.azure\.com)", "https://[REDACTED_AZURE_ENDPOINT]"),
        (r"(?i)((?:api[_-]?key|access[_-]?token|client[_-]?secret)\s*[:=]\s*)\S+", r"\1[REDACTED]"),
    ]
    for pattern, replacement in patterns:
        text = re.sub(pattern, replacement, text)
    if "PRIVATE KEY-----" in text:
        text = "[REDACTED_PRIVATE_KEY_LINE]\n"
    return text


def run_check(args: argparse.Namespace) -> int:
    root = Path(args.root).resolve()
    log = Path(args.log).resolve()
    result_path = Path(args.result).resolve()
    log.parent.mkdir(parents=True, exist_ok=True)
    result_path.parent.mkdir(parents=True, exist_ok=True)
    env, literals = scrub_environment()
    started = utc_now()
    started_monotonic = time.monotonic()
    command = args.command[1:] if args.command and args.command[0] == "--" else args.command
    if not command:
        raise SystemExit("empty command")
    rc = 125
    timed_out = False
    in_private_key = False

    def clean_line(line: str) -> str:
        nonlocal in_private_key
        if "-----BEGIN " in line and "PRIVATE KEY-----" in line:
            in_private_key = True
            return "[REDACTED_PRIVATE_KEY_BLOCK]\n"
        if in_private_key:
            if "-----END " in line and "PRIVATE KEY-----" in line:
                in_private_key = False
            return ""
        return redact(line, literals)

    with log.open("w", encoding="utf-8") as stream:
        try:
            proc = subprocess.Popen(command, cwd=args.cwd, env=env, stdout=subprocess.PIPE,
                                    stderr=subprocess.STDOUT, text=True, errors="replace",
                                    start_new_session=True)
            try:
                output, _ = proc.communicate(timeout=args.timeout)
                rc = int(proc.returncode)
            except subprocess.TimeoutExpired as exc:
                timed_out = True
                os.killpg(proc.pid, signal.SIGTERM)
                try:
                    tail, _ = proc.communicate(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(proc.pid, signal.SIGKILL)
                    tail, _ = proc.communicate()
                prefix = exc.output or ""
                if isinstance(prefix, bytes):
                    prefix = prefix.decode("utf-8", "replace")
                output = prefix + (tail or "")
                rc = 124
            for line in output.splitlines(keepends=True):
                clean = clean_line(line)
                stream.write(clean)
                sys.stdout.write(clean)
            if timed_out:
                message = f"TIMEOUT after {args.timeout} seconds\n"
                stream.write(message)
                sys.stdout.write(message)
            stream.flush(); sys.stdout.flush()
        except OSError as exc:
            stream.write(redact(f"execution failed: {exc}\n", literals))
            rc = 127
    current = fingerprint(root)
    source_unchanged = current["source_sha256"] == args.source_sha256
    if rc == 0 and not source_unchanged:
        rc = 126
        with log.open("a", encoding="utf-8") as stream:
            stream.write("FAIL CLOSED: source fingerprint changed while check ran\n")
    finished = utc_now()
    evidence = {
        "schema_version": 1, "name": args.name,
        "status": "passed" if rc == 0 else "failed", "exit_code": rc,
        "started_at": started, "finished_at": finished,
        "duration_seconds": round(time.monotonic() - started_monotonic, 3),
        "timeout_seconds": args.timeout, "timed_out": timed_out,
        "command": command,
        "command_sha256": hashlib.sha256(json.dumps(command, separators=(",", ":")).encode()).hexdigest(),
        "cwd": str(Path(args.cwd).resolve().relative_to(root)),
        "run_id": args.run_id, "source_sha256": args.source_sha256,
        "source_unchanged": source_unchanged, "git_head": current["git_head"],
        "git_branch": current["git_branch"],
        "log": str(log.relative_to(root)), "log_sha256": sha256_file(log),
        "tool_versions": tool_versions(root),
    }
    result_path.write_text(json.dumps(evidence, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return rc


def checkpoint(args: argparse.Namespace) -> int:
    path = Path(args.result)
    if not path.is_file():
        return 1
    try:
        row = json.loads(path.read_text(encoding="utf-8"))
        log = Path(args.root) / row["log"]
        valid = (row.get("name") == args.name and row.get("status") == "passed"
                 and row.get("exit_code") == 0 and row.get("source_sha256") == args.source_sha256
                 and log.is_file() and sha256_file(log) == row.get("log_sha256"))
        return 0 if valid else 1
    except (OSError, ValueError, KeyError, TypeError):
        return 1


def build_report(args: argparse.Namespace) -> int:
    root = Path(args.root).resolve()
    run_dir = Path(args.run_dir).resolve()
    current = fingerprint(root)
    checks = []
    errors = []
    for name in REQUIRED:
        path = run_dir / "results" / f"{name}.json"
        if not path.is_file():
            checks.append({"name": name, "status": "not_run", "exit_code": None})
            continue
        try:
            row = json.loads(path.read_text(encoding="utf-8"))
            log = root / row["log"]
            valid = (row.get("name") == name and row.get("source_sha256") == current["source_sha256"]
                     and log.is_file() and sha256_file(log) == row.get("log_sha256"))
            if not valid:
                row["status"] = "invalid"
                row["exit_code"] = 125
                errors.append(f"{name}: stale source or log checksum mismatch")
            checks.append(row)
        except (OSError, ValueError, KeyError, TypeError) as exc:
            checks.append({"name": name, "status": "invalid", "exit_code": 125})
            errors.append(f"{name}: unreadable result ({type(exc).__name__})")
    passed = sum(row.get("status") == "passed" and row.get("exit_code") == 0 for row in checks)
    report = {
        "schema_version": 1, "phase": "D", "generated_at": utc_now(),
        "run_id": args.run_id, "status": "passed" if passed == len(REQUIRED) else "failed",
        "exit_code": 0 if passed == len(REQUIRED) else 1,
        "required_check_names": REQUIRED, "passed_count": passed,
        "required_count": len(REQUIRED), "validation_errors": errors,
        "repository": current, "tool_versions_at_report": tool_versions(root),
        "checks": checks,
    }
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return report["exit_code"]


def main() -> int:
    parser = argparse.ArgumentParser()
    subs = parser.add_subparsers(dest="action", required=True)
    fp = subs.add_parser("fingerprint")
    fp.add_argument("--root", required=True)
    run = subs.add_parser("run")
    run.add_argument("--root", required=True); run.add_argument("--name", choices=REQUIRED, required=True)
    run.add_argument("--run-id", required=True); run.add_argument("--cwd", required=True)
    run.add_argument("--log", required=True); run.add_argument("--result", required=True)
    run.add_argument("--source-sha256", required=True); run.add_argument("--timeout", type=int, default=900)
    run.add_argument("command", nargs=argparse.REMAINDER)
    cp = subs.add_parser("checkpoint")
    cp.add_argument("--root", required=True); cp.add_argument("--name", choices=REQUIRED, required=True)
    cp.add_argument("--result", required=True); cp.add_argument("--source-sha256", required=True)
    report = subs.add_parser("build-report")
    report.add_argument("--root", required=True); report.add_argument("--run-dir", required=True)
    report.add_argument("--run-id", required=True); report.add_argument("--output", required=True)
    args = parser.parse_args()
    if args.action == "fingerprint":
        print(json.dumps(fingerprint(Path(args.root).resolve()), indent=2, sort_keys=True)); return 0
    if args.action == "run": return run_check(args)
    if args.action == "checkpoint": return checkpoint(args)
    if args.action == "build-report": return build_report(args)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
