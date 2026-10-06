#!/bin/bash
# Resume this deployment after a guarded rollback; never recreate identities.
set -euo pipefail
stage=${1:?explicit staging directory required}
case "$stage" in /tmp/remoteapp-poc-deploy.*) ;; *) exit 1 ;; esac
nginx=/fm_inetpub/software/nginx/sbin/nginx
nginx_conf=/fm_inetpub/software/nginx/conf/nginx.conf
site=/fm_inetpub/software/nginx/conf/conf.d/remoteapp-poc.conf
test "$(id -u)" = 0
test ! -e "$site"
cmp "$stage/remoteapp-poc.conf" /etc/remoteapp-poc/nginx.disabled.conf
cmp "$stage/config.yaml" /etc/remoteapp-poc/config.yaml
cmp "$stage/policy.json" /etc/remoteapp-poc/policy.json
cmp "$stage/remoteapp-poc.service" /etc/systemd/system/remoteapp-poc.service
sha256sum --check "$stage/existing-config.sha256"
rollback() {
    systemctl stop remoteapp-poc || true
    if test -f "$site"; then
        mv "$site" /etc/remoteapp-poc/nginx.disabled.conf
        "$nginx" -t -c "$nginx_conf" && "$nginx" -s reload -c "$nginx_conf"
    fi
    echo 'FAIL activation_stopped_new_site_disabled_state_preserved'
}
trap rollback ERR
systemctl start remoteapp-poc
for attempt in {1..20}; do
    if curl --silent --fail --noproxy '*' --max-time 3 http://127.0.0.1:18443/health >/dev/null; then break; fi
    sleep 1
done
curl --silent --fail --noproxy '*' --max-time 3 http://127.0.0.1:18443/health >/dev/null
install -m 644 "$stage/remoteapp-poc.conf" "$site"
"$nginx" -t -c "$nginx_conf"
"$nginx" -s reload -c "$nginx_conf"
for attempt in {1..20}; do
    if curl --silent --fail --noproxy '*' --connect-timeout 2 --max-time 3 --resolve mk.fengmap.com:8443:127.0.0.1 https://mk.fengmap.com:8443/health >/dev/null; then break; fi
    sleep 1
done
curl --silent --fail --noproxy '*' --connect-timeout 2 --max-time 3 --resolve mk.fengmap.com:8443:127.0.0.1 https://mk.fengmap.com:8443/health >/dev/null
sha256sum --check "$stage/existing-config.sha256"
trap - ERR
echo 'PASS public_poc_activated_service_not_boot_enabled'
