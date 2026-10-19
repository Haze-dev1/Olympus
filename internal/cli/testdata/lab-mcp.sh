# Minimal stdio MCP fixture. Needs no network or package installation.
# Its credential is required but is never echoed or used outside the process.
[ -n "$GITHUB_TOKEN" ] || exit 1
while IFS= read -r request; do
  id=$(printf '%s\n' "$request" | sed -n 's/.*"id" *: *\([0-9][0-9]*\).*/\1/p')
  [ -n "$id" ] || continue
  case "$request" in
    *'"initialize"'*) result='{"protocolVersion":"2024-11-05","capabilities":{"tools":{}},"serverInfo":{"name":"olympus-lab","version":"1.0.0"}}' ;;
    *'"tools/list"'*) result='{"tools":[]}' ;;
    *) result='{}' ;;
  esac
  printf '{"jsonrpc":"2.0","id":%s,"result":%s}\n' "$id" "$result"
done
