#!/bin/bash
set -e

CONF="/etc/ganesha/ganesha.conf"

# Detect if /volume is an NFS mount by checking /proc/mounts
MOUNT_LINE=$(grep ' /volume ' /proc/mounts | head -1)
FS_TYPE=$(echo "$MOUNT_LINE" | awk '{print $3}')

if [ "$FS_TYPE" = "nfs" ] || [ "$FS_TYPE" = "nfs4" ]; then
    # Extract backend NFS server and path from mount source (e.g., "10.10.20.100:/mnt/storage/path")
    MOUNT_SOURCE=$(echo "$MOUNT_LINE" | awk '{print $1}')
    NFS_SERVER=$(echo "$MOUNT_SOURCE" | cut -d: -f1)
    NFS_PATH=$(echo "$MOUNT_SOURCE" | cut -d: -f2-)

    echo "Detected NFS-backed volume: server=$NFS_SERVER path=$NFS_PATH" >&2
    echo "Using PROXY_V4 FSAL to re-export" >&2

    # Write to /tmp in case /etc is not writable (e.g., ephemeral containers)
    CONF="/tmp/ganesha.conf"
    cat > "$CONF" <<EOF
NFS_Core_Param {
    NFS_Protocols = 4;
    Bind_addr = 0.0.0.0;
    Enable_NLM = false;
    Enable_RQUOTA = false;
}

NFSv4 {
    Grace_Period = 0;
}

EXPORT {
    Export_Id = 1;
    Path = ${NFS_PATH};
    Pseudo = /volume;
    Access_Type = RW;
    Squash = No_Root_Squash;
    SecType = sys;
    Disable_ACL = true;

    FSAL {
        Name = PROXY_V4;
        Srv_Addr = ${NFS_SERVER};
    }
}

LOG {
    Default_Log_Level = WARN;
}
EOF
else
    echo "Detected local/block-backed volume (fstype=$FS_TYPE), using VFS FSAL" >&2
fi

exec /usr/bin/ganesha.nfsd -F -L /dev/stderr -f "$CONF"
