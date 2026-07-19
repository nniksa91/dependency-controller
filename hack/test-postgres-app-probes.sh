#!/usr/bin/env bash
# Compatibility wrapper — scenario renamed to scenario-postgres-app.
# Prefer: ./hack/test-postgres-app-dependency.sh
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/test-postgres-app-dependency.sh" "$@"
