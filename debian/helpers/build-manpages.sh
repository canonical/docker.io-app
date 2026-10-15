#!/bin/bash

# This script generates all the manpages based on the upstream scripts. They
# used to be generated during build time, but now require the download of some
# modules not present in the vendor directory (upstream removed
# documentation-only dependencies).
# Execute this script right after importing a new upstream version to update
# the manpages in debian/manpages.

set -eux

# Create temporary directory to build manpages
build_dir=$(mktemp -d -t docker-cli-docsgen.XXXXXXXXXX)

# Define function to remove the build directory when the script exits
function clean {
  rm -rf "${build_dir}"
}
trap clean EXIT

# Copy the CLI code to the build directory
cp -r cli/ "${build_dir}"
pushd "${build_dir}"/cli

# check if latest golang version available is >= 1.24
LATEST_GO_DIR=$(dpkg -l | grep -Eo golang-[0-9]+\.[0-9]+-go | grep -Eo golang-[0-9]+\.[0-9]+ | sort -ur | head -n1 | sed 's/lang//')
LATEST_GO_VER=$(echo $LATEST_GO_DIR | sed 's/go-//')
MIN_GO_VER="1.24"
if awk "BEGIN {exit ($LATEST_GO_VER < $MIN_GO_VER)}"; then
  # Add the latest Go to the $PATH to be used instead of the default
  # (in case the default is not the latest).
  PATH=/usr/lib/${LATEST_GO_DIR}/bin:"${PATH}"
else
  echo "[E] Please, install golang-x.yz-go. >= 1.24"
  exit 5
fi

# Execute upstream script to generate manpages.
# It will download some Go modules.
# This was taken from the upstream Makefile. with-go-mod creates a stub go.mod
# file to allow some go commands to run. Refer to the Makefile or the scripts
# being rin for further reference.
scripts/with-go-mod.sh scripts/docs/generate-man.sh

popd

# Copy generated manpages to debian/manpages
mkdir -p debian/manpages
cp -r "${build_dir}"/cli/man/man* debian/manpages/

# Create a file containing the upstream version (based on debian/changelog)
# used to build manpages
if ! command -v dpkg-parsechangelog &>/dev/null; then
  echo "[E] Please, install dpkg-dev."
  exit 5
else
  dpkg-parsechangelog | grep '^Version:' | cut -f 2 -d ' ' | cut -f 1 -d '-' \
	  > debian/manpages/upstream-version
fi
