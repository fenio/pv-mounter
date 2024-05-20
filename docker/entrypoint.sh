#!/bin/bash

# Check if SSH_KEY environment variable is set
if [ -n "$SSH_KEY" ]; then
    echo "$SSH_KEY" >> /root/.ssh/authorized_keys
    echo "Public key added to /root/.ssh/authorized_keys"
fi

# Redirect sshd logs to stdout
mkdir -p /var/log/sshd
touch /var/log/sshd/sshd.log
ln -sf /dev/stdout /var/log/sshd/sshd.log

/usr/sbin/sshd -D -e
