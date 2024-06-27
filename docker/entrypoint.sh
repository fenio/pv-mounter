#!/bin/bash

if [ -z "${SSH_PORT}" ]; then
  # Define the variable
  SSH_PORT="2137"
fi

# Determine the user based on NEEDS_ROOT variable
if [ "${NEEDS_ROOT}" = "true" ]; then
  SSH_USER="root"
else
  SSH_USER="ve"
fi

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
        LOG_FILE="/dev/shm/ephemeral_container.log"
        exec > >(tee -a "$LOG_FILE") 2>&1
        /usr/sbin/sshd -D -e -p $SSH_PORT &
        RANDOM_SUFFIX=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 8)
        export SSH_AUTH_SOCK="/dev/shm/ssh-agent-${RANDOM_SUFFIX}.sock"
        eval "$(ssh-agent -a $SSH_AUTH_SOCK)"
        ssh-add <(printf "%s\n" "$SSH_PRIVATE_KEY")
        ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -N -R 2137:localhost:2137 ${SSH_USER}@${PROXY_POD_IP} -p 6666 &
        tail -f /dev/null
        ;;
    *)
        echo "Running default..."
        /usr/sbin/sshd -D -e -p $SSH_PORT
        ;;
esac


