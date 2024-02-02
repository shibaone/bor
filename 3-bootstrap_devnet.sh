#!/bin/sh

set -e

cd ../matic-cli
npm install --prefer-offline --no-audit --progress=false
mkdir devnet
cd devnet
../bin/matic-cli.js setup devnet -c ../../bor/.github/matic-cli-config.yml
