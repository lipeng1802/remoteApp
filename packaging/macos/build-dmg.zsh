#!/bin/zsh

set -euo pipefail

version="${1:-0.1.0}"
script_dir="${0:A:h}"
repo_root="${script_dir:h:h}"
package_dir="${repo_root}/macos/RemoteController"
artifact_dir="${repo_root}/artifacts/macos"
temporary_dir="$(mktemp -d /tmp/prd-macos-package.XXXXXX)"
app_path="${artifact_dir}/RemoteController.app"
dmg_path="${artifact_dir}/PersonalRemoteDesktop-${version}-macOS.dmg"

cleanup() {
    rm -rf "${temporary_dir}"
}
trap cleanup EXIT

mkdir -p "${temporary_dir}/swift-module-cache" "${temporary_dir}/clang-module-cache"
export SWIFTPM_MODULECACHE_OVERRIDE="${temporary_dir}/swift-module-cache"
export CLANG_MODULE_CACHE_PATH="${temporary_dir}/clang-module-cache"

swift build \
    --package-path "${package_dir}" \
    --configuration release \
    --product RemoteController \
    --disable-sandbox \
    --scratch-path "${temporary_dir}/build"

binary_path="${temporary_dir}/build/release/RemoteController"
if [[ ! -x "${binary_path}" ]]; then
    print -u2 "RemoteController binary was not produced at ${binary_path}"
    exit 1
fi

rm -rf "${app_path}" "${dmg_path}"
mkdir -p "${app_path}/Contents/MacOS" "${app_path}/Contents/Resources"
ditto "${binary_path}" "${app_path}/Contents/MacOS/RemoteController"
ditto "${script_dir}/Info.plist" "${app_path}/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString ${version}" "${app_path}/Contents/Info.plist"

signing_identity="${MACOS_SIGNING_IDENTITY:--}"
if [[ "${signing_identity}" == "-" ]]; then
    codesign --force --deep --sign - "${app_path}"
else
    codesign --force --deep --options runtime --timestamp --sign "${signing_identity}" "${app_path}"
fi
codesign --verify --deep --strict --verbose=2 "${app_path}"

mkdir -p "${temporary_dir}/dmg"
ditto "${app_path}" "${temporary_dir}/dmg/RemoteController.app"
ln -s /Applications "${temporary_dir}/dmg/Applications"
hdiutil create \
    -volname "Personal Remote Desktop" \
    -srcfolder "${temporary_dir}/dmg" \
    -format UDZO \
    -ov \
    "${dmg_path}"

print "Created ${dmg_path}"
