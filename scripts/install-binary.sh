#!/usr/bin/env bash
set -euo pipefail

if [ -z "${HELM_PLUGIN_DIR:-}" ]; then
  echo "HELM_PLUGIN_DIR is not set" >&2
  exit 2
fi

mkdir -p "$HELM_PLUGIN_DIR/bin"

echo "Building helm-splice..."
# Build the plugin binary into the plugin's bin/ directory
GOOS=${GOOS:-$(uname | tr '[:upper:]' '[:lower:]')} \
GOARCH=${GOARCH:-amd64} \
  go build -o "$HELM_PLUGIN_DIR/bin/helm-splice" ./cmd/helm-splice

echo "Installed $HELM_PLUGIN_DIR/bin/helm-splice"
