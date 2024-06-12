FROM debian:stable-slim

# Update package list and install necessary packages
RUN apt-get update && \
    apt-get install -y --no-install-recommends openssh-server openssh-client && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir /var/run/sshd /volume

# Copy scripts and configuration files
COPY entrypoint.sh /entrypoint.sh
COPY sshkey.sh /sshkey.sh
COPY sshd_config /etc/ssh/sshd_config

# Create user and set permissions
RUN groupadd -r -g 2137 ve && \
    useradd -m -r -s /bin/bash -u 2137 -g ve ve && \
    chmod +x /entrypoint.sh /sshkey.sh && \
    chown -R ve:ve /var/run/sshd /run /volume /entrypoint.sh /etc/ssh

# Expose port
EXPOSE 2137

# Switch to non-root user
USER ve

# Entrypoint
ENTRYPOINT ["/entrypoint.sh"]

