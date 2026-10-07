#!/bin/bash
# Independent device-only pre-release service. No public listener/proxy yet.
# Never upload release-local-only or reuse/overwrite an existing installation.
set -euo pipefail
stage=${1:?explicit stage required}
release_root=${2:?public pinned release root required}
binary_hash=${3:?binary SHA256 required}
case "$stage" in /tmp/remoteapp-device-deploy.*) ;; *) exit 1 ;; esac
[[ "$release_root" =~ ^[0-9a-f]{64}$ ]]
[[ "$binary_hash" =~ ^[0-9a-f]{64}$ ]]
test "$(id -u)" = 0
test ! -e "$stage/release-local-only"
for target in /opt/remoteapp-prerelease /var/lib/remoteapp-prerelease /etc/systemd/system/remoteapp-device.service; do
    test ! -e "$target" || { echo 'FAIL prerelease_target_exists'; exit 1; }
done
! id remoteapp-pre >/dev/null 2>&1
! ss -lnt | grep -Eq ':8444[[:space:]]'
test "$(sha256sum "$stage/device-service-linux" | cut -d ' ' -f 1)" = "$binary_hash"
test -f "$stage/server-private/authority/state.json"
test ! -e "$stage/server-private/authority/writer.lock"
test -f "$stage/client-public/service-profile.json"
# Read only hashes of existing Nginx configuration, not TLS key material.
sha256sum /fm_inetpub/software/nginx/conf/nginx.conf /fm_inetpub/software/nginx/conf/conf.d/*.conf > "$stage/nginx-before.sha256"
useradd --system --no-create-home --home-dir /var/lib/remoteapp-prerelease --shell /sbin/nologin remoteapp-pre
install -d -m 755 /opt/remoteapp-prerelease
install -m 755 "$stage/device-service-linux" /opt/remoteapp-prerelease/device-service
install -d -m 700 -o remoteapp-pre -g remoteapp-pre /var/lib/remoteapp-prerelease
cp -a "$stage/server-private" "$stage/client-public" /var/lib/remoteapp-prerelease/
chown -R remoteapp-pre:remoteapp-pre /var/lib/remoteapp-prerelease/server-private /var/lib/remoteapp-prerelease/client-public
# Template contains no secrets; replace only the public operator-pinned root.
sed "s/@RELEASE_ROOT@/$release_root/" "$stage/remoteapp-device.service" > /etc/systemd/system/remoteapp-device.service
chmod 644 /etc/systemd/system/remoteapp-device.service
systemctl daemon-reload
trap 'systemctl stop remoteapp-device || true; echo FAIL_new_service_stopped_state_preserved' ERR
systemctl start remoteapp-device
systemctl is-active --quiet remoteapp-device
sha256sum --check "$stage/nginx-before.sha256" >/dev/null
trap - ERR
echo 'PASS device_only_deployed_loopback_not_boot_enabled; public_8444_not_opened'
