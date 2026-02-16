#!/bin/bash
set -e

# All writable paths go to /tmp so this works as both root and non-root
# (ephemeral containers inherit the pod's user which may be non-root)
mkdir -p /tmp/ganesha
CONF="/tmp/ganesha/ganesha.conf"

# Detect if /volume is an NFS mount by checking /proc/mounts
MOUNT_LINE=$(grep ' /volume ' /proc/mounts | head -1)
FS_TYPE=$(echo "$MOUNT_LINE" | awk '{print $3}')

# FORCE_VFS=true skips PROXY_V4 and uses VFS FSAL even for NFS-backed volumes.
# This is needed for ephemeral containers running as non-root, where PROXY_V4
# can't use reserved ports required by most NFS servers.
# VFS FSAL rejects NFS mounts by checking /etc/mtab, so we create a modified
# mtab that presents /volume as a local filesystem. The actual I/O goes through
# the kernel VFS layer which handles NFS transparently.
if [ "$FORCE_VFS" = "true" ]; then
    echo "FORCE_VFS set, using VFS FSAL (fstype=$FS_TYPE)" >&2
    if [ "$FS_TYPE" = "nfs" ] || [ "$FS_TYPE" = "nfs4" ]; then
        DEVICE=$(echo "$MOUNT_LINE" | awk '{print $1}')
        OPTIONS=$(echo "$MOUNT_LINE" | awk '{print $4}')
        grep -v ' /volume ' /proc/mounts > /tmp/ganesha/mtab
        echo "$DEVICE /volume ext4 $OPTIONS 0 0" >> /tmp/ganesha/mtab
        ln -sf /tmp/ganesha/mtab /etc/mtab
        echo "Created synthetic /etc/mtab to present /volume as ext4" >&2
    fi
    FSAL_BLOCK="FSAL { Name = VFS; }"
    EXPORT_PATH="Path = /volume; Filesystem_id = 1.1;"
elif [ "$FS_TYPE" = "nfs" ] || [ "$FS_TYPE" = "nfs4" ]; then
    # Extract backend NFS server and path from mount source (e.g., "10.10.20.100:/mnt/storage/path")
    MOUNT_SOURCE=$(echo "$MOUNT_LINE" | awk '{print $1}')
    NFS_SERVER=$(echo "$MOUNT_SOURCE" | cut -d: -f1)
    NFS_PATH=$(echo "$MOUNT_SOURCE" | cut -d: -f2-)

    echo "Detected NFS-backed volume: server=$NFS_SERVER path=$NFS_PATH" >&2
    echo "Using PROXY_V4 FSAL to re-export" >&2

    FSAL_BLOCK="FSAL { Name = PROXY_V4; Srv_Addr = ${NFS_SERVER}; }"
    EXPORT_PATH="Path = ${NFS_PATH};"
else
    echo "Detected local/block-backed volume (fstype=$FS_TYPE), using VFS FSAL" >&2

    FSAL_BLOCK="FSAL { Name = VFS; }"
    EXPORT_PATH="Path = /volume; Filesystem_id = 1.1;"
fi

cat > "$CONF" <<EOF
NFS_Core_Param {
    NFS_Protocols = 4;
    Bind_addr = 0.0.0.0;
    Enable_NLM = false;
    Enable_RQUOTA = false;
    allow_set_io_flusher_fail = true;
}

NFSv4 {
    Grace_Period = 0;
    RecoveryBackend = fs_ng;
    RecoveryRoot = /tmp/ganesha;
}

EXPORT {
    Export_Id = 1;
    ${EXPORT_PATH}
    Pseudo = /volume;
    Access_Type = RW;
    Squash = No_Root_Squash;
    SecType = sys;
    Disable_ACL = true;

    ${FSAL_BLOCK}
}

LOG {
    Default_Log_Level = ${LOG_LEVEL:-WARN};
}
EOF

exec /usr/bin/ganesha.nfsd -F -L /proc/self/fd/2 -p /tmp/ganesha/ganesha.pid -f "$CONF"
