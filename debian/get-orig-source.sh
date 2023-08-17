#!/bin/bash

set -eux -o pipefail

if [ "$1" = '--upstream-version' ]; then
    VERSION="$2"
else
    printf "E: missing argument '--upstream-version'.\n" 1>&2
    exit 1
fi

# Check if the script is executed from the root of the source package
if [ ! -d debian ]; then
    printf "E: 'debian' directory not found.\n" 1>&2
    exit 1
fi

COMPONENTS=(
"docker/cli=v$( debian/helpers/real-upstream-version.sh "$VERSION" )"
"krallin/tini=v$( debian/helpers/tini-upstream-version.sh "$VERSION" )"
)

DEB_SOURCE="$( dpkg-parsechangelog -SSource )"
filename="$( readlink -f ../${DEB_SOURCE}_${VERSION}.orig.tar.gz )"
[ -s "${filename}" ] || exit 1

# prepare a workdir
work_dir="$( mktemp -d -t get-orig-source_${DEB_SOURCE}_XXXXXXXX )"
trap "rm -rf '${work_dir}'" EXIT

# extract main tarball
tar -xf "${filename}" -C "${work_dir}"

# move sources in a subdirectory
mv "${work_dir}/moby-${VERSION}" "${work_dir}/engine"

# fetch Docker components
for I in "${COMPONENTS[@]}"; do
  echo "COMPONENT => $I"
  C=$(   echo ${I} | awk -F= '{print $1}' )
  REV=$( echo ${I} | awk -F= '{print $2}' )
  URL="github.com/${C}"
  COMP=${C##*/}
  FN="${work_dir}/$COMP.tar.gz"

  # download tarball:
  archive_url="https://${URL}/archive/${REV}.tar.gz"
  wget --quiet --tries=3 --timeout=40 --read-timeout=40 --continue \
      -O "${FN}" "$archive_url"

  # extract tarball:
  tar -xf "${FN}" -C "${work_dir}"
  mv "${work_dir}/${COMP}-${REV##*v}" "${work_dir}/${COMP}"
  rm ${FN}
done

# create final tarball
pushd "${work_dir}"
tar cJf "${filename}" "."

# notify the developer about update of the manpages
if [ $? -eq 0 ]; then
  echo "#########################################################################"
  echo "DO NOT FORGET TO RUN debian/helpers/build-manpages.sh TO UPDATE MANPAGES!"
  echo "For more information, please read debian/README.source"
  echo "#########################################################################"
fi
popd
