#!/bin/bash

# Grab tini version used in a given moby/moby upstream version.

set -eu

if [ $# -ne 1 ]; then
    echo >&2 "Usage: $0 UPSTREAM_VERSION"
    exit 1
fi

MOBY_URL="https://raw.githubusercontent.com/moby/moby"
TINI_FILE="hack/dockerfile/install/tini.installer"
DEB_SOURCE="$( dpkg-parsechangelog -SSource )"

work_dir="$( mktemp -d -t get-tini-version_${DEB_SOURCE}_XXXXXXXX )"
trap "rm -rf '${work_dir}'" EXIT

cd $work_dir
wget --quiet -nv "$MOBY_URL/v$1/$TINI_FILE"
cat tini.installer | grep "TINI_VERSION:=" | cut -d 'v' -f2 | cut -d '}' -f1
