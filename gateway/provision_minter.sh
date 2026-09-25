#!/bin/bash
set +x
set -euo pipefail
umask 077

if [ -z "${YOURPLACE_MINTER_CREDENTIALS:-}" ]; then
  echo 'No minter credentials configured; signing remains paused.'
  exit 0
fi
: "${INSTANCE_ID:?INSTANCE_ID is required}"
AWS_REGION="${AWS_REGION:-us-east-1}"
command -v session-manager-plugin >/dev/null || { echo 'Install the AWS Session Manager plugin before provisioning.'; exit 1; }
jq -en 'env.YOURPLACE_MINTER_CREDENTIALS | fromjson | type == "object"' >/dev/null 2>&1 || { echo 'Invalid minter credential JSON.'; exit 1; }

session_log=$(mktemp)
tunnel_pid=''
cleanup() {
  if [ -n "$tunnel_pid" ]; then
    kill "$tunnel_pid" 2>/dev/null || true
    wait "$tunnel_pid" 2>/dev/null || true
  fi
  session_id=$(sed -n 's/^Starting session with SessionId: \([a-zA-Z0-9_-]*\).*$/\1/p' "$session_log" | head -n 1)
  if [ -n "$session_id" ]; then
    aws ssm terminate-session --session-id "$session_id" --region "$AWS_REGION" >/dev/null 2>&1 || true
  fi
  rm -f "$session_log"
  unset YOURPLACE_MINTER_CREDENTIALS
}
trap cleanup EXIT
trap 'exit 1' INT TERM

aws ssm start-session --target "$INSTANCE_ID" --region "$AWS_REGION" \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["42430"],"localPortNumber":["42431"]}' >"$session_log" 2>&1 &
tunnel_pid=$!
ready=0
for attempt in $(seq 1 30); do
  if ! kill -0 "$tunnel_pid" 2>/dev/null; then break; fi
  if grep -q '^Port 42431 opened for sessionId ' "$session_log" && curl --noproxy '*' --fail --silent --max-time 2 http://127.0.0.1:42431/health >/dev/null; then ready=1; break; fi
  sleep 2
done
if [ "$ready" -ne 1 ]; then echo 'Minter bootstrap tunnel unavailable.'; exit 1; fi
jq -cn '{credentials: (env.YOURPLACE_MINTER_CREDENTIALS | fromjson)}' 2>/dev/null | \
  curl --noproxy '*' --fail --silent --show-error --max-time 60 \
    -H 'Content-Type: application/json' --data-binary @- \
    http://127.0.0.1:42431/credentials >/dev/null
echo 'Minter credentials provisioned into Server memory.'
