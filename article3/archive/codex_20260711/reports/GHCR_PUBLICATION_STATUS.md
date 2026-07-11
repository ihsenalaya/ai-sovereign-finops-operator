# GHCR Publication Status

## Summary

The Article 3 local Docker build and local Kind plus Helm validation completed
successfully, but GHCR publication is still blocked.

## Evidence

- `gh auth status` succeeds for the local CLI session.
- `docker build -f article3/Dockerfile.experiment -t gov-ar-experiment:local .`
  succeeded.
- `gh auth token | docker login ghcr.io -u ihsenalaya --password-stdin`
  returned `denied` from the GHCR registry endpoint during the publication
  attempt on 2026-07-11.

## Interpretation

This strongly suggests that the current GitHub authentication context does not
present effective `packages:write` permission for GHCR publication, even though
the CLI session itself is valid.

## Impact

- local build validation: complete
- local Helm packaging: complete
- local Kind execution: complete
- remote GHCR push: blocked pending a GitHub session with package write access

## Recommended next unblock step

Refresh or replace the GitHub authentication context with a token or session
that includes package publication rights, then retry the immutable Article 3
tag push and record the resulting GHCR digest in
`article3/provenance/image_digests.csv`.
