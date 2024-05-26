FROM debian:stable-slim

RUN apt-get update && \
    apt-get install -y openssh-server openssh-client && \
    mkdir /var/run/sshd && \
    mkdir /volume 

COPY entrypoint.sh /entrypoint.sh
COPY sshd_config /etc/ssh/sshd_config
RUN chmod +x /entrypoint.sh

EXPOSE 2137

ENTRYPOINT ["/entrypoint.sh"]
