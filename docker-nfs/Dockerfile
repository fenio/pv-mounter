### Stage 1: Build NFS-Ganesha 9.5 from source
FROM debian:stable-slim AS builder

RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections
RUN apt-get update && apt-get install -y --no-install-recommends \
    git cmake gcc g++ make \
    bison flex \
    libdbus-1-dev liburcu-dev libkrb5-dev libcap-dev \
    libnfsidmap-dev libblkid-dev uuid-dev libnsl-dev \
    libjemalloc-dev libacl1-dev \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
RUN git clone --branch V9.5 --depth 1 --recurse-submodules \
    https://github.com/nfs-ganesha/nfs-ganesha.git

WORKDIR /build/nfs-ganesha/src

# Prevent Ganesha from dropping capabilities at startup.
# lower_my_caps() replaces all capabilities with a minimal set, which
# strips the DAC_READ_SEARCH/DAC_OVERRIDE caps needed for non-root
# ephemeral containers to access volume files.
RUN sed -i 's/lower_my_caps();/\/\/ lower_my_caps(); \/\/ patched: keep container caps/' MainNFSD/nfs_main.c

RUN mkdir build && cd build && cmake \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX=/usr \
    -DUSE_FSAL_VFS=ON \
    -DUSE_FSAL_PROXY_V4=ON \
    -DUSE_FSAL_PROXY_V3=OFF \
    -DUSE_FSAL_LUSTRE=OFF \
    -DUSE_FSAL_LIZARDFS=OFF \
    -DUSE_FSAL_KVSFS=OFF \
    -DUSE_FSAL_CEPH=OFF \
    -DUSE_FSAL_GPFS=OFF \
    -DUSE_FSAL_XFS=OFF \
    -DUSE_FSAL_GLUSTER=OFF \
    -DUSE_FSAL_NULL=OFF \
    -DUSE_FSAL_RGW=OFF \
    -DUSE_FSAL_MEM=OFF \
    -DUSE_FSAL_SAUNAFS=OFF \
    -DUSE_DBUS=OFF \
    -DUSE_NFSIDMAP=ON \
    -DUSE_9P=OFF \
    -DUSE_MONITORING=OFF \
    -DUSE_GSS=OFF \
    -DUSE_GUI_ADMIN_TOOLS=OFF \
    -DUSE_ADMIN_TOOLS=OFF \
    .. && \
    make -j$(nproc) && \
    make DESTDIR=/install install

### Stage 2: Runtime image
FROM debian:stable-slim

RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    liburcu8t64 libkrb5-3 libcap2 libnfsidmap1 \
    libblkid1 libuuid1 libjemalloc2 libacl1 libdbus-1-3 libnsl2 && \
    apt-get clean && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /volume /run/ganesha && \
    rm -f /etc/mtab && touch /etc/mtab && chmod 666 /etc/mtab

COPY --from=builder /install/usr /usr

# Copy configuration and entrypoint
COPY ganesha.conf /etc/ganesha/ganesha.conf
COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

# Expose NFS port
EXPOSE 2049

# Entrypoint
ENTRYPOINT ["/entrypoint.sh"]
