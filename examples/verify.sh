#!/bin/sh
# Terminal 1: record the original provider through any SDK or curl.
set -eu

equal record --target https://api.example.com --cassette account.jsonl

# Terminal 2, while the recorder is running:
# curl http://127.0.0.1:8080/v1/account -H "Authorization: Bearer $TEST_TOKEN"
#
# Stop the recorder, review account.jsonl, then replay without the provider:
# equal replay --cassette account.jsonl
#
# Against a candidate implementation instead:
# equal verify --target http://127.0.0.1:9000 --cassette account.jsonl \
#   --ignore-json '/request_id' --json compatibility-report.json
