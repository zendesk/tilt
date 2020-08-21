#!/bin/bash
#
# Build a Docker container with all the cross-compiling toolchains
# we need to do a release. Pre-populate it with a Go cache.

set -ex

DIR=$(dirname "$0")
cd "$DIR/.."

docker build -t tilt-releaser-base -f scripts/release.Dockerfile scripts
docker run --name tilt-releaser-go-cache --privileged \
       -w /src/tilt \
       -v "$PWD:/src/tilt:delegated" \
       tilt-releaser-base \
       -v /var/run/docker.sock:/var/run/docker.sock \
       --rm-dist --skip-validate --snapshot --skip-publish
docker commit tilt-releaser-go-cache gcr.io/windmill-public-containers/tilt-releaser
docker push gcr.io/windmill-public-containers/tilt-releaser
docker container rm tilt-releaser-go-cache
