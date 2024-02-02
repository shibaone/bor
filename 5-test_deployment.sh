#!/bin/sh

set -e

echo "Deposit 100 matic for each account to bor network"
cd ../matic-cli/devnet/code/contracts
npm run truffle exec scripts/deposit.js -- --network development $(jq -r .root.tokens.MaticToken contractAddresses.json) 100000000000000000000
cd -
timeout 60m bash integration-tests/smoke_test.sh
