#!/bin/bash

# Check if SSH_KEY environment variable is set
if [ -n "$SSH_KEY" ]; then
    echo "$SSH_KEY" >> /root/.ssh/authorized_keys
    echo "Public key added to /root/.ssh/authorized_keys"
fi

# Check the ROLE environment variable
case "$ROLE" in
    standalone)
        echo "Running as standalone"
        /usr/sbin/sshd -D -e
        ;;
    proxy)
        echo "Running as proxy"
        /usr/sbin/sshd -D -e
        ;;
    ephemeral)
        echo "Running as ephemeral"
        /usr/sbin/sshd -D-e
        ;;
    *)
        echo "Running default..."
        /usr/sbin/sshd -D -e
        ;;
esac

