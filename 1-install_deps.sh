#!/bin/sh

set -e

sudo apt update
sudo DEBIAN_FRONTEND=noninteractive apt install -y build-essential
curl https://raw.githubusercontent.com/creationix/nvm/master/install.sh | bash
sudo snap install solc
sudo DEBIAN_FRONTEND=noninteractive apt install -y python2 jq curl

wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz -O go.tar.gz
sudo tar -C /usr/local -xzvf go.tar.gz

cat >> ~/.bashrc << EOF
export GOROOT=/usr/local/go
export GOPATH=\$HOME/go
export PATH=\$GOPATH/bin:\$GOROOT/bin:\$PATH
EOF

curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER
newgrp docker
