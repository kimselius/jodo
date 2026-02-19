#!/bin/sh
# Copy mounted public key into authorized_keys with correct permissions
if [ -f /tmp/jodo_key.pub ]; then
    cp /tmp/jodo_key.pub /root/.ssh/authorized_keys
    chmod 600 /root/.ssh/authorized_keys
    chown root:root /root/.ssh/authorized_keys
fi

# Generate host keys if missing (first boot)
ssh-keygen -A 2>/dev/null

# Auto-start seed.py if it exists (resumes after container restart)
# This prevents the kernel health checker from escalating to nuclear rebirth
# before seed.py has a chance to boot.
if [ -f /opt/jodo/brain/seed.py ]; then
    echo "[entrypoint] Found seed.py — auto-starting (resume after restart)"
    cd /opt/jodo/brain && nohup python3 /opt/jodo/brain/seed.py > /var/log/jodo.log 2>&1 &
fi

exec /usr/sbin/sshd -D
