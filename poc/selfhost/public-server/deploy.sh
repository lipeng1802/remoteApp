#!/bin/bash
# Run as root on the inspected CentOS 7 PoC host, after uploading all five files.
# Intentionally refuses a second deployment rather than overwriting state.
set -euo pipefail
stage=${1:?explicit staging directory required}
case "$stage" in /tmp/remoteapp-poc-deploy.*) ;; *) exit 1 ;; esac
test "$(id -u)" = 0
nginx=/fm_inetpub/software/nginx/sbin/nginx
nginx_conf=/fm_inetpub/software/nginx/conf/nginx.conf
site=/fm_inetpub/software/nginx/conf/conf.d/remoteapp-poc.conf
for target in /opt/remoteapp-poc /etc/remoteapp-poc /var/lib/remoteapp-poc /etc/systemd/system/remoteapp-poc.service "$site"; do
    test ! -e "$target" || { echo 'FAIL deployment_target_exists'; exit 1; }
done
! id remoteapp-poc >/dev/null 2>&1
! ss -lntu | grep -Eq ':(8443|3478|18443|19090|15443)[[:space:]]'
test "$(sha256sum "$stage/headscale-0.29.4-linux-amd64" | cut -d ' ' -f 1)" = 212ed0a884c0d3541e094c4bebbe94397df6f4e01bd3d7f059c520cb55e0d757
"$nginx" -t -c "$nginx_conf"
# Capture only configuration hashes, never the TLS private key.
sha256sum "$nginx_conf" /fm_inetpub/software/nginx/conf/conf.d/mk.fengmap.com.conf > "$stage/existing-config.sha256"
useradd --system --no-create-home --home-dir /var/lib/remoteapp-poc --shell /sbin/nologin remoteapp-poc
install -d -m 755 /opt/remoteapp-poc
install -d -m 750 -o root -g remoteapp-poc /etc/remoteapp-poc
install -d -m 700 -o remoteapp-poc -g remoteapp-poc /var/lib/remoteapp-poc
install -m 755 "$stage/headscale-0.29.4-linux-amd64" /opt/remoteapp-poc/headscale
/opt/remoteapp-poc/headscale version
install -m 640 -o root -g remoteapp-poc "$stage/config.yaml" "$stage/policy.json" /etc/remoteapp-poc/
install -m 644 "$stage/remoteapp-poc.service" /etc/systemd/system/remoteapp-poc.service
systemctl daemon-reload
rollback() {
    systemctl stop remoteapp-poc || true
    if test -f "$site"; then
        mv "$site" /etc/remoteapp-poc/nginx.disabled.conf
        "$nginx" -t -c "$nginx_conf" && "$nginx" -s reload -c "$nginx_conf"
    fi
    echo 'FAIL deployment_stopped_new_site_disabled_state_preserved'
}
trap rollback ERR
systemctl start remoteapp-poc
for attempt in {1..20}; do
    if curl --silent --fail --noproxy '*' http://127.0.0.1:18443/health >/dev/null; then break; fi
    sleep 1
done
curl --silent --fail --noproxy '*' http://127.0.0.1:18443/health >/dev/null
systemctl is-active --quiet remoteapp-poc
install -m 644 "$stage/remoteapp-poc.conf" "$site"
"$nginx" -t -c "$nginx_conf"
"$nginx" -s reload -c "$nginx_conf"
# Nginx reload returns before the replacement workers open the new listener.
for attempt in {1..20}; do
    if curl --silent --fail --noproxy '*' --connect-timeout 2 --max-time 3 --resolve mk.fengmap.com:8443:127.0.0.1 https://mk.fengmap.com:8443/health >/dev/null; then break; fi
    sleep 1
done
curl --silent --fail --noproxy '*' --connect-timeout 2 --max-time 3 --resolve mk.fengmap.com:8443:127.0.0.1 https://mk.fengmap.com:8443/health >/dev/null
sha256sum --check "$stage/existing-config.sha256"
trap - ERR
echo 'PASS public_poc_deployed_service_not_boot_enabled'
