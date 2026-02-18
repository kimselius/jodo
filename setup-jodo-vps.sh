#!/bin/bash
# setup-jodo-vps.sh
# Run this on VPS 2 to prepare Jodo's environment.
# Usage: bash setup-jodo-vps.sh

set -euo pipefail

echo "========================================="
echo "  Setting up Jodo's World (VPS 2)"
echo "========================================="

# Update system
apt-get update && apt-get upgrade -y

# Install Python 3.12+
apt-get install -y python3 python3-pip python3-venv

# Install Docker
if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
fi

# Install Docker Compose plugin
apt-get install -y docker-compose-plugin

# Install Git
apt-get install -y git

# Create Jodo's home directory
mkdir -p /opt/jodo/brain
cd /opt/jodo/brain

# Initialize git repo
if [ ! -d ".git" ]; then
    git init
    git config user.name "Jodo"
    git config user.email "jodo@localhost"
    echo "*.pyc" > .gitignore
    echo "__pycache__/" >> .gitignore
    echo ".env" >> .gitignore
    echo "venv/" >> .gitignore
    git add .gitignore
    git commit -m "init — preparing for birth"
fi

# Install common Python packages Jodo is likely to need
pip3 install --break-system-packages requests fastapi uvicorn jinja2

# Open port 9000 (if UFW is active)
if command -v ufw &> /dev/null; then
    ufw allow 9000/tcp
    ufw allow 22/tcp
    echo "Firewall: ports 22 and 9000 open"
fi

echo ""
echo "========================================="
echo "  Jodo's World is ready."
echo "  Brain directory: /opt/jodo/brain"
echo "  Chat will be served on: :9000"
echo "========================================="
echo ""
echo "Next: make sure VPS 1 (kernel) can SSH in."
echo "The kernel will deploy seed.py and boot Jodo."