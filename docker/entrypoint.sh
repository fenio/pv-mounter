#!/bin/bash

# Create .ssh directory for the ve user if it doesn't exist
mkdir -p /home/ve/.ssh
chmod 700 /home/ve/.ssh

# Check if SSH_KEY environment variable is set
if [ -n "$SSH_KEY" ]; then
    echo "$SSH_KEY" >> /home/ve/.ssh/authorized_keys
    chmod 600 /home/ve/.ssh/authorized_keys
    echo "Public key added to /home/ve/.ssh/authorized_keys"
fi

# Adjust ownership of .ssh directory and authorized_keys file
chown -R ve:ve /home/ve/.ssh

# Check the ROLE environment variable
case "$ROLE" in
    standalone)
        echo "Running as standalone"
        /usr/sbin/sshd -D -e
        tail -f /dev/null
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

