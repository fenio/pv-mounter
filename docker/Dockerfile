FROM debian:stable-slim@sha256:f6681102cd18b4c0c4720a77b602498f4bdcf701c8fc02776dfb0d4c350c381f

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
COPY sshd_config.standard /etc/ssh/sshd_config

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
