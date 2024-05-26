#!/bin/bash

# Create .ssh directory for the ve user if it doesn't exist
mkdir -p /home/ve/.ssh
chmod 700 /home/ve/.ssh

if [ -z "${SSH_PORT}" ]; then
  # Define the variable
  SSH_PORT="2137"
fi

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
        /usr/sbin/sshd -D -e -p $SSH_PORT
        tail -f /dev/null
        ;;
    proxy)
        echo "Running as proxy"
        /usr/sbin/sshd -D -e -p $SSH_PORT
        ;;
    ephemeral)
        echo "Running as ephemeral"
        echo "$SSH_PRIVATE_KEY" > /home/ve/.ssh/id_rsa
        echo "$SSH_PUBLIC_KEY" > /home/ve/.ssh/authorized_keys
        chmod 600 /home/ve/.ssh/id_rsa
        /usr/sbin/sshd -e -p $SSH_PORT
        ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -N -R 2137:localhost:2137 ve@${PROXY_POD_IP} -p 6666
        tail -f /dev/null
        ;;
    *)
        echo "Running default..."
        /usr/sbin/sshd -D -e -p $SSH_PORT
        ;;
esac

