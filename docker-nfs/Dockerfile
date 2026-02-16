FROM debian:stable-slim@sha256:4448d44b91bf4a13cb1b4e02d9d5f87ed40621d6e33f0ae7b6ddf71d57e29364

# Update package list and install NFS-Ganesha
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections
RUN apt-get update && \
    apt-get install -y --no-install-recommends nfs-ganesha nfs-ganesha-vfs && \
    apt-get clean && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /volume /run/ganesha && \
    ln -sf /proc/mounts /etc/mtab

# Copy configuration and entrypoint
COPY ganesha.conf /etc/ganesha/ganesha.conf
COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

# Expose NFS port
EXPOSE 2049

# Entrypoint
ENTRYPOINT ["/entrypoint.sh"]
