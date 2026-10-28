#!/bin/zsh
# Packages a TEST transaction directory without changing any executable bytes.
set -euo pipefail
if (( $# != 2 )); then
    print -u2 'Usage: build-native-dmg.zsh absolute-transaction-directory absolute-new-output.dmg'
    exit 1
fi
payload="$1"
output="$2"
[[ "$payload" == /* && -d "$payload" && "$output" == /* && ! -e "$output" ]] || exit 1
[[ -x "${payload}/installer-adapter" && -x "${payload}/install-transaction" && -d "${payload}/release-payload" && -d "${payload}/bootstrap-payload" ]] || exit 1
script_dir="${0:A:h}"
stage="$(mktemp -d "${TMPDIR:-/private/tmp}/remoteapp-native-dmg.XXXXXX")"
trap '/bin/rm -rf -- "$stage"' EXIT
/usr/bin/ditto "$payload" "${stage}/payload"
/usr/bin/ditto "${script_dir}/Install.command" "${stage}/Install.command"
/bin/chmod 0700 "${stage}/Install.command"
/usr/bin/hdiutil create -volname 'RemoteApp Native Preview' -srcfolder "$stage" -format UDZO -ov "$output"
/usr/bin/hdiutil verify "$output"
/usr/bin/shasum -a 256 "$output"
