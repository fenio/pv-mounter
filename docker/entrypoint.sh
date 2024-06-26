#!/bin/bash

if [ -z "${SSH_PORT}" ]; then
  # Define the variable
  SSH_PORT="2137"
fi

# Check if the container should run as root
if [ "$NEEDS_ROOT" = "true" ]; then
    echo "Running as root"
    user="root"
    group="root"
else
    echo "Running as non-root user"
    user="ve"
    group="ve"
fi

# Ensure correct permissions for SSH host keys and necessary directories
chmod 600 /etc/ssh/ssh_host_*
chown -R $user:$group /var/run/sshd /run /volume /etc/ssh

# Check the ROLE environment variable
case "$ROLE" in
    standalone)
        echo "Running as standalone"
        /usr/sbin/sshd -D -e -p $SSH_PORT
        ;;
    proxy)
        echo "Running as proxy"
        /usr/sbin/sshd -D -e -p $SSH_PORT
        ;;
    ephemeral)
        echo "Running as ephemeral"
        /usr/sbin/sshd -e -p $SSH_PORT
        RANDOM_SUFFIX=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 8)
        export SSH_AUTH_SOCK="/dev/shm/ssh-agent-${RANDOM_SUFFIX}.sock"
        eval "$(ssh-agent -a $SSH_AUTH_SOCK)"
        ssh-add <(printf "%s\n" "$SSH_PRIVATE_KEY")
        ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -N -R 2137:localhost:2137 ve@${PROXY_POD_IP} -p 6666 &
        tail -f /dev/null
        ;;
    *)
        echo "Running default..."
        /usr/sbin/sshd -D -e -p $SSH_PORT
        ;;
esac

