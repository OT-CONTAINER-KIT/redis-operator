#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CRD_REF_DOCS_VERSION="v0.0.12"

echo "Starting API documentation generation..."

# Install crd-ref-docs if not present
echo "Installing crd-ref-docs ${CRD_REF_DOCS_VERSION}..."
go install github.com/elastic/crd-ref-docs@${CRD_REF_DOCS_VERSION}

# Use full path to crd-ref-docs
GOPATH=$(go env GOPATH)
if [ -z "$GOPATH" ]; then
    GOPATH="$HOME/go"
fi
CRD_REF_DOCS="${GOPATH}/bin/crd-ref-docs"

if [ ! -f "${CRD_REF_DOCS}" ]; then
    echo "Error: crd-ref-docs not found at ${CRD_REF_DOCS}"
    exit 1
fi

# Create output directory
OUTPUT_DIR="${REPO_ROOT}/docs/content/en/docs/CRD Reference/API Reference"
mkdir -p "${OUTPUT_DIR}"

echo "Generating API documentation..."
echo "Source path: ${REPO_ROOT}/api"
echo "Config file: ${SCRIPT_DIR}/config.yaml"
echo "Templates: ${SCRIPT_DIR}/templates"
echo "Output: ${OUTPUT_DIR}/api-reference.md"

# Generate the documentation using default templates
"${CRD_REF_DOCS}" \
    --source-path="${REPO_ROOT}/api" \
    --config="${SCRIPT_DIR}/config.yaml" \
    --renderer=markdown \
    --output-path="${OUTPUT_DIR}/api-reference.md"

echo "API documentation generated successfully at: ${OUTPUT_DIR}/api-reference.md"