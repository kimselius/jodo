FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    openssh-server git curl && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /var/run/sshd /root/.ssh && chmod 700 /root/.ssh

# Enable SSH with key-based auth
RUN sed -i 's/#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config

# Create brain directory
RUN mkdir -p /opt/jodo/brain

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 22 9000 9001

ENTRYPOINT ["/entrypoint.sh"]
