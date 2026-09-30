#!/bin/zsh

set -euo pipefail

script_dir="${0:A:h}"
repo_root="${script_dir:h:h}"
version="${1:-$(<"${repo_root}/VERSION")}"
package_dir="${repo_root}/macos/RemoteController"
artifact_dir="${repo_root}/artifacts/macos"
app_path="${artifact_dir}/RemoteController.app"
dmg_name="PersonalRemoteDesktop-${version}-macOS.dmg"
dmg_path="${artifact_dir}/${dmg_name}"

if ! print -r -- "${version}" | /usr/bin/grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
    print -u2 "Version must use numeric major.minor.patch format: ${version}"
    exit 1
fi

relevant_changes="$(git -C "${repo_root}" status --porcelain -- macos/RemoteController packaging/macos VERSION)"
if [[ -n "${relevant_changes}" ]]; then
    print -u2 "Commit or remove macOS package input changes before building a traceable artifact."
    print -u2 -- "${relevant_changes}"
    exit 1
fi

source_revision="$(git -C "${repo_root}" rev-parse --short=12 HEAD)"
temporary_dir="$(mktemp -d /tmp/prd-macos-package.XXXXXX)"

cleanup() {
    rm -rf "${temporary_dir}"
}
trap cleanup EXIT

mkdir -p "${temporary_dir}/swift-module-cache" "${temporary_dir}/clang-module-cache"
export SWIFTPM_MODULECACHE_OVERRIDE="${temporary_dir}/swift-module-cache"
export CLANG_MODULE_CACHE_PATH="${temporary_dir}/clang-module-cache"

swift test \
    --package-path "${package_dir}" \
    --configuration release \
    --disable-sandbox \
    --scratch-path "${temporary_dir}/build"

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

rm -rf "${app_path}" "${dmg_path}" "${dmg_path}.sha256"
mkdir -p "${app_path}/Contents/MacOS" "${app_path}/Contents/Resources"
ditto "${binary_path}" "${app_path}/Contents/MacOS/RemoteController"
ditto "${script_dir}/Info.plist" "${app_path}/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString ${version}" "${app_path}/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion ${version}" "${app_path}/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :PRDSourceRevision ${source_revision}" "${app_path}/Contents/Info.plist"

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
hdiutil verify "${dmg_path}"

(
    cd "${artifact_dir}"
    /usr/bin/shasum -a 256 "${dmg_name}" > "${dmg_name}.sha256"
)

print "Created ${dmg_path}"
print "Checksum ${dmg_path}.sha256"
