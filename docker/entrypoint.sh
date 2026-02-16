#!/bin/bash

SSH_PORT="${SSH_PORT:-2137}"

if [ "${NEEDS_ROOT}" = "true" ]; then
  SSHD_CONFIG="/etc/ssh/sshd_config.privileged"
else
  SSHD_CONFIG="/etc/ssh/sshd_config.standard"
fi

/usr/sbin/sshd -D -e -p "$SSH_PORT" -f "$SSHD_CONFIG"
