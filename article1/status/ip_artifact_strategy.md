# IP / Artifact Strategy — Article 1

Generated: 2026-07-05

## Status

`PRIVATE_ARTIFACT_UNTIL_IP_REVIEW`

The placement-token and signed `AIPlacementDecision` mechanisms may be
IP-sensitive. The paper and README must not promise immediate public release
until IP review is complete.

## Submission-Safe Wording

Use:

```text
The artifact is available as a private review package pending IP review.
```

Avoid:

```text
The complete implementation is publicly available.
```

## Review Package Contents

The private package may include:

- source code with secrets removed;
- Helm charts and Terraform without subscription IDs or tfstate;
- raw result CSV/logs with no kubeconfig or private keys;
- scripts to regenerate figures;
- Overleaf/LaTeX source.

## Must Not Include

- kubeconfig;
- service principal secrets;
- GHCR token;
- OpenAI/API keys;
- scheduler private signing key;
- Terraform state;
- Azure subscription ID if avoidable.

## Q1 Impact

Medium. This does not invalidate the science, but it constrains artifact
availability claims and must be explicit in threats-to-validity or artifact
availability notes.
