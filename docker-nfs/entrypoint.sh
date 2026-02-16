#!/bin/bash

# Bind-mount /volume onto itself so it appears as a distinct mount point
# in /proc/mounts. Ganesha's VFS FSAL requires the export path to be
# resolvable via the mount table.
mount --bind /volume /volume

exec /usr/bin/ganesha.nfsd -F -L /dev/stderr -f /etc/ganesha/ganesha.conf
