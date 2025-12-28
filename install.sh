#!/usr/bin/env bash
#
# install.sh
#
# This script is used to install the protolog logging facility as a systemd
# service. It will accept a password input needed to gain the sudo privileged
# necessary for installing all dependencies
#
# This script is designed to work on AWS Linux. The user must make prot 8080
# accessible for HTTP requests from their device in order to reach the hosted
# web server
#
# This script expect to be run from the protolog home directory

set -euo pipefail

# Required argument to the script is the desired working directory of the
# registered systemd service
if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <working-directory>"
  exit 1
fi

###
# CONSTANTS
###

WORKDIR="$1"
SERVICE_NAME="protolog-collector.service"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}"
PWD_DIR="$PWD"
EXEC_START="${PWD_DIR}/bin/log-collector"

###
# INSTALL DEPENDENCIES
###

sudo dnf -y update

# golang
sudo dnf -y install golang

# ZeroMQ
sudo dnf -y install zeromq zeromq-devel

# Protobuf
sudo dnf -y install protobuf protobuf-compiler protobuf-devel

# Protobuf-c
sudo dnf -y install protobuf-c protobuf-c-compiler protobuf-c-devel

# protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
export PATH=$PATH:$HOME/go/bin

# npm
sudo dnf install -y nodejs

# Typescript
sudo npm install -g typescript

###
# COMPILATION
###

# Compile the necessary internal protobuf messages for go and python
make proto
make py-proto

# Compile the go code
go build -o bin/log-collector ./cmd/log-collector

# Compile the ui
cd ui/web
npm run build
cd ../..

###
# INSTALL SERVICE
###

sudo tee "$SERVICE_FILE" > /dev/null <<EOF
[unit]
Description=Protolog log collector and web UI
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ec2-user
WorkingDirectory=${WORKDIR}

ExecStart=${EXEC_START} \\
  -addr tcp://*:5556 \\
  -http-addr :8080

Restart=on-failure
RestartSec=5

Environment=PATH=/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"
sudo systemctl restart "$SERVICE_NAME"

echo "Installed protolog and started new systemd service ${SERVICE_NAME}"
