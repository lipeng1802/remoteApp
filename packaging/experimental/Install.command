#!/bin/zsh
set -euo pipefail
if (( $# != 0 )); then
    print -u2 'This installer does not accept arguments.'
    exit 1
fi
package_dir="${0:A:h}"
if "${package_dir}/payload/installer-adapter"; then
    print 'Installation confirmed. You may eject this disk image.'
else
    result=$?
    print -u2 'Installation was not confirmed. Recovery state was retained.'
    exit "${result}"
fi
