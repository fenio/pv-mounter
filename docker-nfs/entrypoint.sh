#!/bin/bash

echo "=== /volume contents ===" >&2
ls -la /volume >&2 || echo "/volume does not exist" >&2

echo "=== /proc/mounts ===" >&2
cat /proc/mounts >&2

echo "=== Attempting bind mount ===" >&2
mount --bind /volume /volume 2>&1 >&2
echo "bind mount exit code: $?" >&2

echo "=== /proc/mounts after bind ===" >&2
cat /proc/mounts >&2

exec /usr/bin/ganesha.nfsd -F -L /dev/stderr -f /etc/ganesha/ganesha.conf
