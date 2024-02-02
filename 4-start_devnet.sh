#!/bin/sh

set -e

cd ../matic-cli/devnet
bash docker-ganache-start.sh
bash docker-heimdall-start-all.sh
bash docker-bor-setup.sh
bash docker-bor-start-all.sh
docker compose  up -d kms
cd -
timeout 2m bash integration-tests/bor_health.sh
cd -
bash ganache-deployment-bor.sh
bash ganache-deployment-sync.sh
