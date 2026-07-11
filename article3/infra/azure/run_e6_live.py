#!/usr/bin/env python3
"""Run the live Azure validation for Article 3 E6 without persisting secrets."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def run(*args: str) -> str:
    proc = subprocess.run(
        args,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        check=True,
    )
    return proc.stdout


def run_curl(url: str, api_key: str, payload: dict) -> dict:
    proc = subprocess.run(
        [
            "curl",
            "--http1.1",
            "-sS",
            "-o",
            "-",
            "-w",
            "\n%{http_code}",
            url,
            "-H",
            f"api-key: {api_key}",
            "-H",
            "Content-Type: application/json",
            "-d",
            json.dumps(payload),
        ],
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )
    text = proc.stdout
    if "\n" in text:
        body, status = text.rsplit("\n", 1)
    else:
        body, status = text, ""
    try:
        parsed = json.loads(body) if body else {}
    except Exception:
        parsed = {"raw": body[:1000]}
    return {
        "status_code": status,
        "body": parsed,
        "curl_exit_code": proc.returncode,
    }


def summarize(result: dict) -> dict:
    body = result.get("body", {})
    summary = {
        "status_code": result.get("status_code"),
        "curl_exit_code": result.get("curl_exit_code"),
    }
    if isinstance(body, dict):
        if "choices" in body and body["choices"]:
            msg = body["choices"][0].get("message", {})
            summary["content"] = msg.get("content")
            summary["usage"] = body.get("usage")
        elif "output_text" in body:
            summary["content"] = body.get("output_text")
            summary["usage"] = body.get("usage")
        elif "error" in body and body["error"] is not None:
            summary["error"] = body["error"]
        elif "raw" in body:
            summary["raw"] = body["raw"]
    return summary


def main() -> int:
    account = json.loads(run("az", "account", "show"))
    openai_resources = json.loads(
        run(
            "az",
            "resource",
            "list",
            "--resource-type",
            "Microsoft.CognitiveServices/accounts",
            "-o",
            "json",
        )
    )

    france_meta = json.loads(
        run(
            "az",
            "cognitiveservices",
            "account",
            "deployment",
            "show",
            "-g",
            "rg-article2-governance-20260708",
            "-n",
            "a2fr60f9b020260708",
            "--deployment-name",
            "gpt-france-mini",
            "-o",
            "json",
        )
    )
    us_meta = json.loads(
        run(
            "az",
            "cognitiveservices",
            "account",
            "deployment",
            "show",
            "-g",
            "rg-article2-governance-20260708",
            "-n",
            "a2us60f9b020260708",
            "--deployment-name",
            "gpt-us-mini",
            "-o",
            "json",
        )
    )
    mistral_meta = json.loads(
        run(
            "az",
            "cognitiveservices",
            "account",
            "deployment",
            "show",
            "-g",
            "rg-greenops-fresh-20260708",
            "-n",
            "greenops-fdry-60f9b0",
            "--deployment-name",
            "mistral-large-latest",
            "-o",
            "json",
        )
    )

    france_key = run(
        "az",
        "cognitiveservices",
        "account",
        "keys",
        "list",
        "-g",
        "rg-article2-governance-20260708",
        "-n",
        "a2fr60f9b020260708",
        "--query",
        "key1",
        "-o",
        "tsv",
    ).strip()
    us_key = run(
        "az",
        "cognitiveservices",
        "account",
        "keys",
        "list",
        "-g",
        "rg-article2-governance-20260708",
        "-n",
        "a2us60f9b020260708",
        "--query",
        "key1",
        "-o",
        "tsv",
    ).strip()
    mistral_key = run(
        "az",
        "cognitiveservices",
        "account",
        "keys",
        "list",
        "-g",
        "rg-greenops-fresh-20260708",
        "-n",
        "greenops-fdry-60f9b0",
        "--query",
        "key1",
        "-o",
        "tsv",
    ).strip()

    prompt = "Return exactly OK."
    attempts = {
        "france_deployment_chat_2024_10_21": run_curl(
            "https://a2fr60f9b020260708.openai.azure.com/openai/deployments/gpt-france-mini/chat/completions?api-version=2024-10-21",
            france_key,
            {"messages": [{"role": "user", "content": prompt}], "max_tokens": 5},
        ),
        "us_deployment_chat_2024_10_21": run_curl(
            "https://a2us60f9b020260708.openai.azure.com/openai/deployments/gpt-us-mini/chat/completions?api-version=2024-10-21",
            us_key,
            {"messages": [{"role": "user", "content": prompt}], "max_tokens": 5},
        ),
        "france_v1_responses": run_curl(
            "https://a2fr60f9b020260708.openai.azure.com/openai/v1/responses",
            france_key,
            {"model": "gpt-france-mini", "input": prompt},
        ),
        "us_v1_responses": run_curl(
            "https://a2us60f9b020260708.openai.azure.com/openai/v1/responses",
            us_key,
            {"model": "gpt-us-mini", "input": prompt},
        ),
        "mistral_services_openai_v1_chat": run_curl(
            "https://greenops-fdry-60f9b0.services.ai.azure.com/openai/v1/chat/completions",
            mistral_key,
            {"model": "mistral-large-latest", "messages": [{"role": "user", "content": prompt}], "max_tokens": 5},
        ),
        "mistral_services_models_chat_preview": run_curl(
            "https://greenops-fdry-60f9b0.services.ai.azure.com/models/chat/completions?api-version=2024-05-01-preview",
            mistral_key,
            {"model": "mistral-large-latest", "messages": [{"role": "user", "content": prompt}], "max_tokens": 5},
        ),
    }

    output = {
        "experiment_id": "E6",
        "variant": "azure_live_discovery_and_invocation",
        "subscription": {
            "id": account.get("id"),
            "name": account.get("name"),
            "tenant_id": account.get("tenantId"),
        },
        "resource_count": len(openai_resources),
        "resources": [
            {
                "name": r.get("name"),
                "resource_group": r.get("resourceGroup"),
                "location": r.get("location"),
                "kind": r.get("kind"),
            }
            for r in openai_resources
        ],
        "deployments": {
            "gpt_france_mini": {
                "account": "a2fr60f9b020260708",
                "location": france_meta["resourceGroup"],
                "model": france_meta["properties"]["model"],
                "capabilities": france_meta["properties"]["capabilities"],
            },
            "gpt_us_mini": {
                "account": "a2us60f9b020260708",
                "location": us_meta["resourceGroup"],
                "model": us_meta["properties"]["model"],
                "capabilities": us_meta["properties"]["capabilities"],
            },
            "mistral_large_latest": {
                "account": "greenops-fdry-60f9b0",
                "location": mistral_meta["resourceGroup"],
                "model": mistral_meta["properties"]["model"],
                "capabilities": mistral_meta["properties"]["capabilities"],
            },
        },
        "attempts": {name: summarize(result) for name, result in attempts.items()},
    }

    raw_path = ROOT / "experiments" / "raw" / "E6_azure_live.json"
    processed_path = ROOT / "experiments" / "processed" / "E6_azure_live_summary.json"
    report_path = ROOT / "reports" / "E6_AZURE_LIVE_RESULTS.md"
    raw_path.parent.mkdir(parents=True, exist_ok=True)
    processed_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    raw_path.write_text(json.dumps(output, indent=2), encoding="utf-8")
    processed_path.write_text(json.dumps(output, indent=2), encoding="utf-8")

    lines = [
        "# E6 Azure Live Results",
        "",
        "Live discovery and invocation attempts against Azure resources already present in the subscription.",
        "",
        "```json",
        json.dumps(output, indent=2),
        "```",
        "",
    ]
    report_path.write_text("\n".join(lines), encoding="utf-8")
    print(json.dumps(output, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
