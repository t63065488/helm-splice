#!/usr/bin/env bash
set -euo pipefail

if [ -z "${HELM_PLUGIN_DIR:-}" ]; then
  echo "HELM_PLUGIN_DIR is not set" >&2
  exit 2
fi

PLUGIN_DIR="$HELM_PLUGIN_DIR"
cd "$PLUGIN_DIR"

mkdir -p "$PLUGIN_DIR/bin"

echo "Building helm-splice..."
# Build the plugin binary into the plugin's bin/ directory
GOOS=${GOOS:-$(go env GOOS)} \
GOARCH=${GOARCH:-$(go env GOARCH)} \
  go build -o "$PLUGIN_DIR/bin/helm-splice" ./cmd/helm-splice

echo "Installed $PLUGIN_DIR/bin/helm-splice"
