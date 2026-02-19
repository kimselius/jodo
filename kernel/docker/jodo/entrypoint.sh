#!/bin/sh
# Copy mounted public key into authorized_keys with correct permissions
if [ -f /tmp/jodo_key.pub ]; then
    cp /tmp/jodo_key.pub /root/.ssh/authorized_keys
    chmod 600 /root/.ssh/authorized_keys
    chown root:root /root/.ssh/authorized_keys
fi

# Generate host keys if missing (first boot)
ssh-keygen -A 2>/dev/null

exec /usr/sbin/sshd -D
