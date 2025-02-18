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

# Check if golang-1.24-go is installed
if dpkg -l golang-1.24-go >/dev/null 2>&1; then
  # Add Go 1.24 to the $PATH to be used instead of the default
  # (in case the default is not 1.24).
  PATH=/usr/lib/go-1.24/bin:"${PATH}"
else
  echo "[E] Please, install golang-1.21-go."
  exit 5
fi

# Place built binaries in ${build_dir} instead of /tmp
sed -i s#/tmp/gen-manpages#"${build_dir}"/gen-manpages#g ./scripts/docs/generate-man.sh
sed -i s#/tmp/go-md2man#"${build_dir}"/go-md2man#g ./scripts/docs/generate-man.sh

# Execute upstream script to generate manpages.
# It will download some Go modules.
./scripts/docs/generate-man.sh

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
