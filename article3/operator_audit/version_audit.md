# Version audit

- Fetched remote base: `origin/main` = `07cdd3baad26abfa7248dd69cdd507aab4be8177`.
- Operator chart and app version at that SHA: 0.5.11.
- Umbrella chart and app version at that SHA: 0.5.11.
- Remote controller 0.5.11 resolves as index digest `sha256:abc591624156aa2a6dc958221d3c0968a1fed11475d6a8e2b38c6b6644a62e88`.
- OCI umbrella chart 0.5.11 pulls and its `.tgz` SHA-256 is `7a21d883b7725b8321af24ad612078d6f952712134697f362a768f966d795db9`.
- Fetched tags: v0.5.4, v0.5.3, v0.4.0, v0.3.9, v0.1.0. No fetched v0.5.11 tag exists; the coherent remote release is untagged.
- Local `main` is not selected: its operator subchart reached 0.5.17 while the umbrella chart/image/README remained 0.5.11.
- GOV-AR is branch-local, introduced principally in commit `51f1124`, with no chart/app version bump and no published `gov-ar-admission:0.5.11` image.

The experimental release must use a new SemVer prerelease and immutable commit/run tag; it must not overwrite or claim operator 0.5.11 artifacts.
