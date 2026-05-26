#!/bin/bash

# register-plugins.sh — Register all built plugins via gRPC CreatePlugin API.
# Requires a running easyp service and grpcurl installed.
#
# Usage: ./register-plugins.sh [host:port]
#   host:port defaults to localhost:8080

set -euo pipefail

GRPC_HOST="${1:-localhost:8080}"
PLUGINS_PREFIX="${2:-/plugins}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PLUGINS_DIR="${SCRIPT_DIR}/plugins"

if ! command -v grpcurl &>/dev/null; then
    echo "Error: grpcurl is not installed. Install it: https://github.com/fullstorydev/grpcurl" >&2
    exit 1
fi

if [ ! -d "${PLUGINS_DIR}" ]; then
    echo "Warning: plugins directory does not exist: ${PLUGINS_DIR}" >&2
    echo "Run ./build-plugins.sh first."
    exit 0
fi

plugin_count=0

for plugin_bin in $(find "${PLUGINS_DIR}" -name plugin -type f | sort); do
    # Extract relative path: plugins/{group}/{name}/{version}/plugin
    rel_path="${plugin_bin#"${PLUGINS_DIR}/"}"
    dir_path=$(dirname "${rel_path}")

    # Parse group/name/version
    if [[ "${dir_path}" =~ ^([^/]+)/([^/]+)/(.+)$ ]]; then
        group="${BASH_REMATCH[1]}"
        name="${BASH_REMATCH[2]}"
        version="${BASH_REMATCH[3]}"

        echo "Registering ${group}/${name}:${version}..."

        # Call CreatePlugin via grpcurl, capture output and exit code
        output=$(grpcurl -plaintext \
            -d "{\"group\":\"${group}\",\"name\":\"${name}\",\"version\":\"${version}\",\"config\":{\"command\":[\"${PLUGINS_PREFIX}/${group}/${name}/${version}/plugin\"]}}" \
            "${GRPC_HOST}" \
            api.generator.v1.ServiceAPI/CreatePlugin 2>&1) && rc=0 || rc=$?

        if [ "${rc}" -ne 0 ]; then
            if echo "${output}" | grep -qi "AlreadyExists\|ALREADY_EXISTS\|already exists"; then
                echo "⚠ Already exists: ${group}/${name}:${version}"
            else
                echo "✗ Failed to register ${group}/${name}:${version}" >&2
                echo "${output}" >&2
                exit 1
            fi
        else
            echo "✓ Registered ${group}/${name}:${version}"
        fi

        plugin_count=$((plugin_count + 1))
    fi
done

if [ "${plugin_count}" -eq 0 ]; then
    echo "Warning: no plugins found in ${PLUGINS_DIR}" >&2
fi

echo ""
echo "Done! Processed ${plugin_count} plugin(s)."
