FROM debian:stable-slim

RUN apt-get update && \
    apt-get install -y openssh-server openssh-client && \
    mkdir /var/run/sshd && \
    mkdir /volume 

COPY entrypoint.sh /entrypoint.sh
COPY sshd_config /etc/ssh/sshd_config
RUN groupadd -r -g 2137 ve && useradd -m -r -s /bin/bash -u 2137 -g ve ve
RUN chmod +x /entrypoint.sh && \
    chown -R ve:ve /var/run/sshd /run /volume /entrypoint.sh /etc/ssh

EXPOSE 2137

ENTRYPOINT ["/entrypoint.sh"]
