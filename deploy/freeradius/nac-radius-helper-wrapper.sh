#!/bin/sh
set -eu

. /etc/nac/radius-helper.env
export NAC_RADIUS_API_TOKEN NAC_RADIUS_AUTHORIZE_URL NAC_RADIUS_ACCOUNTING_URL
export NAC_RADIUS_HELPER_TIMEOUT_SECONDS

exec /usr/local/libexec/nac-radius-helper "$@"
