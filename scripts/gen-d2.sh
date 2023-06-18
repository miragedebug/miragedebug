#!/usr/bin/env bash

set -e

CUR_DIR=$(
    cd -- "$(dirname "$0")" >/dev/null 2>&1
    pwd -P
)

for f in $(find ${CUR_DIR}/.. -name "*.d2"); do
    args=$(sed -n 's/^# d2-args:\s*\(.*\)/\1/p' $f)
    d2 ${args} ${f} ${f}.svg
done
