#!/bin/bash
#
# Run goreleaser in a Docker container with all the cross-compiling toolchains
# we need to do a release.

set -ex

if [[ "$GITHUB_TOKEN" == "" ]]; then
    echo "Missing GITHUB_TOKEN"
    exit 1
fi

DIR=$(dirname "$0")
cd "$DIR/.."

docker pull gcr.io/windmill-public-containers/tilt-releaser
docker run --rm --privileged \
       -e GITHUB_TOKEN="$GITHUB_TOKEN" \
       -w /src/tilt \
       -v "$PWD:/src/tilt:delegated" \
       -v ~/.docker:/root/.docker \
       -v /var/run/docker.sock:/var/run/docker.sock \
       gcr.io/windmill-public-containers/tilt-releaser \
       --rm-dist --skip-validate --snapshot --skip-publish
