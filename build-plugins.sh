#!/bin/bash

# build-plugins.sh — Build all plugin binaries from registry/ Dockerfiles.
# Uses docker build --output to extract binaries directly to ./plugins/.
#
# Usage: ./build-plugins.sh

set -euo pipefail

export DOCKER_BUILDKIT=1

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REGISTRY_DIR="${SCRIPT_DIR}/registry"
PLUGINS_DIR="${SCRIPT_DIR}/plugins"

echo "Building plugins from ${REGISTRY_DIR}..."

for dockerfile in $(find "${REGISTRY_DIR}" -name Dockerfile | sort); do
    dir=$(dirname "${dockerfile}")
    # Extract relative path: registry/{group}/{name}/{version}
    rel_path="${dir#"${REGISTRY_DIR}/"}"

    # Parse group/name/version from path
    if [[ "${rel_path}" =~ ^([^/]+)/([^/]+)/(.+)$ ]]; then
        group="${BASH_REMATCH[1]}"
        name="${BASH_REMATCH[2]}"
        version="${BASH_REMATCH[3]}"

        output_dir="${PLUGINS_DIR}/${group}/${name}/${version}"
        mkdir -p "${output_dir}"

        echo "Building ${group}/${name}:${version}..."
        docker build --output="${output_dir}/" "${dir}"

        chmod +x "${output_dir}/plugin"
        echo "✓ Built ${group}/${name}:${version} → ${output_dir}/plugin"
    else
        echo "✗ Skipping unexpected path: ${rel_path}" >&2
        exit 1
    fi
done

echo ""
echo "Done! All plugins built in ${PLUGINS_DIR}/"
