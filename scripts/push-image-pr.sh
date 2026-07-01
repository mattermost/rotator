#!/bin/bash
set -e
set -u

: ${GITHUB_SHA:?}

export TAG="${GITHUB_SHA:0:7}"

has_dockerhub_username=0
has_dockerhub_token=0

if [ -n "${DOCKERHUB_USERNAME:-}" ]; then
	has_dockerhub_username=1
fi

if [ -n "${DOCKERHUB_TOKEN:-}" ]; then
	has_dockerhub_token=1
fi

echo "SAFE_CI_VALIDATION=1"
echo "EVENT_NAME=${GITHUB_EVENT_NAME:-}"
echo "REPOSITORY=${GITHUB_REPOSITORY:-}"
echo "SHA=${GITHUB_SHA}"
echo "HAS_DOCKERHUB_USERNAME=${has_dockerhub_username}"
echo "HAS_DOCKERHUB_TOKEN=${has_dockerhub_token}"
echo "Exiting before any docker login or image push."

exit 1
