#!/bin/bash

cd $(dirname $0)/..

buildDateString=$(date -u '+%Y-%m-%dT%H:%M:%S')
versionShortHash=$(git rev-parse --short HEAD)
versionHash=$(git rev-parse HEAD)
versionBuildNumber=$(git rev-list --all --count $versionHash)

# If we have any changes, append "-dev" so we know the hash is bogus
if ! $(git diff --exit-code >/dev/null); then
    versionShortHash="$versionShortHash-dev"
fi

go build -ldflags "\
    -X github.com/derezzolution/plex-playlister/version.buildDateString=$buildDateString \
    -X github.com/derezzolution/plex-playlister/version.versionShortHash=$versionShortHash \
    -X github.com/derezzolution/plex-playlister/version.versionHash=$versionHash \
    -X github.com/derezzolution/plex-playlister/version.versionBuildNumber=$versionBuildNumber"
