FROM debian:stable-slim@sha256:85dfcffff3c1e193877f143d05eaba8ae7f3f95cb0a32e0bc04a448077e1ac69

# Update package list and install necessary packages
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections
RUN apt-get update && \
    apt-get install -y --no-install-recommends openssh-server openssh-client && \
    apt-get clean && \
    apt-get autoremove -y && \
    rm -f /usr/bin/ssh-keyscan && \
    rm -f /usr/bin/ssh-keygen && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir /volume

# Copy scripts and configuration files
COPY entrypoint.sh /entrypoint.sh
COPY sshkey.sh /sshkey.sh
COPY sshd_config.standard /etc/ssh/sshd_config.standard
COPY sshd_config.privileged /etc/ssh/sshd_config.privileged

# Create user and set permissions
# Host keys in /etc/ssh/ stay root-owned (for privileged/root mode)
# Copy host keys to /etc/ssh/ve_keys/ owned by ve (for non-root mode)
RUN groupadd -r -g 2137 ve && \
    useradd -m -r -s /bin/bash -u 2137 -g ve ve && \
    chmod +x /entrypoint.sh /sshkey.sh && \
    mkdir -p /etc/ssh/ve_keys && \
    cp /etc/ssh/ssh_host_* /etc/ssh/ve_keys/ && \
    chown -R ve:ve /etc/ssh/ve_keys /volume

# Expose port
EXPOSE 2137

# Entrypoint
ENTRYPOINT ["/entrypoint.sh"]
