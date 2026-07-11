#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-gov-ar}"
kind delete cluster --name "${CLUSTER_NAME}"
