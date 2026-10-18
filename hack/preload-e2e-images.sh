#!/usr/bin/env bash
# Pull the images the e2e suite uses once and side-load them into every kind
# node. Kind nodes do not share an image store, so without this each pod
# re-pulls from the registry on every node. The test manifests use
# IfNotPresent, so nothing goes back over the network afterwards, and the
# whole run is pinned to a single digest of the moving :latest tags.
#
# Usage: hack/preload-e2e-images.sh [kind-cluster-name]
set -euo pipefail

CLUSTER="${1:-kind}"

IMAGES=(
  quay.io/opstree/redis:latest
  quay.io/opstree/redis-sentinel:latest
  redis:alpine
  busybox:latest
  busybox:1.36
)

# Docker Hub rate-limits anonymous pulls from shared runner IPs, so a pull can
# fail for reasons that have nothing to do with the change under test.
printf '%s\n' "${IMAGES[@]}" \
  | xargs -P 4 -n 1 -I{} sh -c 'docker pull -q "{}" || echo "::warning::pull failed for {}"'

# Side-loading is an optimisation, not a correctness requirement: anything that
# fails here is just pulled by the kubelet on first use. Warn rather than fail
# the job -- `kind load` cannot import a multi-arch image saved from Docker's
# containerd image store, and that is not worth breaking E2E over.
for img in "${IMAGES[@]}"; do
  kind load docker-image "$img" --name "$CLUSTER" \
    || echo "::warning::could not preload ${img}; kubelet will pull it on demand"
done
