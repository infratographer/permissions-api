#!/bin/sh
# script to bootstrap a nats operator environment

if go tool nsc describe operator; then
    echo "operator exists, not overwriting config"
    exit 0
fi

echo "Cleaning up NATS environment"
rm -rf /nsc/*

echo "Creating NATS operator"
go tool nsc add operator --generate-signing-key --sys --name LOCAL
go tool nsc edit operator -u 'nats://nats:4222'
go tool nsc list operators
go tool nsc describe operator

export OPERATOR_SIGNING_KEY_ID=`go tool nsc describe operator -J | jq -r '.nats.signing_keys | first'`

echo "Creating NATS account for permissions-api"
go tool nsc add account -n INFRADEV -K ${OPERATOR_SIGNING_KEY_ID}
go tool nsc edit account INFRADEV --sk generate --js-mem-storage -1 --js-disk-storage -1 --js-streams -1 --js-consumer -1
go tool nsc describe account INFRADEV

export ACCOUNTS_SIGNING_KEY_ID=`go tool nsc describe account INFRADEV -J | jq -r '.nats.signing_keys | first'`

echo "Creating NATS user for permissions-api"
go tool nsc add user -n USER -K ${ACCOUNTS_SIGNING_KEY_ID}
go tool nsc describe user USER

echo "Generating NATS resolver.conf"
go tool nsc generate config --mem-resolver --sys-account SYS --config-file /nats/resolver.conf --force
