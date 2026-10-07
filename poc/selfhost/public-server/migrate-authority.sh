#!/bin/bash
# Existing live device-only installation only. Never restores stale state.
set -euo pipefail
stage=${1:?stage required}
root=${2:?public release root required}
ah=${3:?authority binary hash required}
fh=${4:?frontend binary hash required}
case "$stage" in /tmp/remoteapp-authority-deploy.*) ;; *) exit 1;; esac
[[ "$root" =~ ^[0-9a-f]{64}$ && "$ah" =~ ^[0-9a-f]{64}$ && "$fh" =~ ^[0-9a-f]{64}$ ]]
test "$(id -u)" = 0
for path in /var/lib/remoteapp-authority /etc/remoteapp-authority /run/remoteapp-authority /etc/systemd/system/remoteapp-authority.service /etc/systemd/system/remoteapp-frontend.service /etc/tmpfiles.d/remoteapp-authority.conf; do
 test ! -e "$path"
done
test "$(sha256sum "$stage/authority-daemon" | cut -d ' ' -f1)" = "$ah"
test "$(sha256sum "$stage/device-frontend" | cut -d ' ' -f1)" = "$fh"
systemctl is-active --quiet remoteapp-device
uid=$(id -u remoteapp-pre)
gid=$(id -g remoteapp-pre)
test -f /var/lib/remoteapp-prerelease/server-private/authority/state.json
sha256sum -c /opt/remoteapp-prerelease/nginx-baseline.sha256 >/dev/null
sed -e "s/@RELEASE_ROOT@/$root/" -e "s/@FRONTEND_UID@/$uid/" -e "s/@FRONTEND_GID@/$gid/" "$stage/remoteapp-authority.service" > "$stage/authority.service"
sed "s/@RELEASE_ROOT@/$root/" "$stage/remoteapp-frontend.service" > "$stage/frontend.service"
systemctl stop remoteapp-device
trap 'systemctl stop remoteapp-frontend remoteapp-authority || true; echo FAIL_stopped_latest_state_preserved_no_automatic_rollback' ERR
! ss -lnt | grep -Eq ':8444[[:space:]]'
test ! -e /var/lib/remoteapp-prerelease/server-private/authority/writer.lock
backup=$(mktemp -d /var/lib/remoteapp-authority-backup.XXXXXXXX)
chmod 700 "$backup"
cp -a /var/lib/remoteapp-prerelease/server-private/authority "$backup/authority"
chown -R root:root "$backup"
cp -a /opt/remoteapp-prerelease/device-service "$backup/device-service"
cp -a /etc/systemd/system/remoteapp-device.service "$backup/remoteapp-device.service"
sha256sum /var/lib/remoteapp-prerelease/server-private/authority/state.json > "$backup/state-before.sha256"
stat -c '%U %G %a %n' /var/lib/remoteapp-prerelease/server-private/authority /var/lib/remoteapp-prerelease/server-private/authority/state.json > "$backup/permissions-before.txt"
install -d -m700 /etc/remoteapp-authority
install -m600 /var/lib/remoteapp-prerelease/client-public/service-profile.json /etc/remoteapp-authority/service-profile.json
mv /var/lib/remoteapp-prerelease/server-private/authority /var/lib/remoteapp-authority
chown -R root:root /var/lib/remoteapp-authority
chmod 700 /var/lib/remoteapp-authority
chmod 600 /var/lib/remoteapp-authority/state.json
install -d -m710 -o root -g remoteapp-pre /run/remoteapp-authority
printf 'd /run/remoteapp-authority 0710 root remoteapp-pre -\n' > /etc/tmpfiles.d/remoteapp-authority.conf
chmod 644 /etc/tmpfiles.d/remoteapp-authority.conf
install -m755 "$stage/authority-daemon" /opt/remoteapp-prerelease/authority-daemon
install -m755 "$stage/device-frontend" /opt/remoteapp-prerelease/device-frontend
install -m644 "$stage/authority.service" /etc/systemd/system/remoteapp-authority.service
install -m644 "$stage/frontend.service" /etc/systemd/system/remoteapp-frontend.service
systemctl daemon-reload
systemctl start remoteapp-authority
for attempt in $(seq 1 50); do
 test ! -S /run/remoteapp-authority/authority.sock || break
 sleep .1
done
systemctl is-active --quiet remoteapp-authority
test -S /run/remoteapp-authority/authority.sock
systemctl start remoteapp-frontend
for attempt in $(seq 1 50); do
 ss -lnt | grep -Eq '127.0.0.1:8444[[:space:]]' && break
 sleep .1
done
systemctl is-active --quiet remoteapp-frontend
ss -lnt | grep -Eq '127.0.0.1:8444[[:space:]]'
test ! -e /var/lib/remoteapp-prerelease/server-private/authority
test "$(cut -d ' ' -f1 "$backup/state-before.sha256")" = "$(sha256sum /var/lib/remoteapp-authority/state.json | cut -d ' ' -f1)"
sha256sum -c /opt/remoteapp-prerelease/nginx-baseline.sha256 >/dev/null
trap - ERR
echo "PASS device_only_split_migrated; backup=$backup; not_boot_enabled"
