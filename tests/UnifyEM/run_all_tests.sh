#!/bin/bash

#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

# Runs all UnifyEM tests and writes timestamped logs.

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

if [ ! -f "$TESTS_DIR/.env" ]; then
    echo "Error: .env file not found in $TESTS_DIR"
    echo "Please create a .env file with APIKEY=your-api-token and SERVER_URL=your-server-url"
    exit 1
fi

FAILED=0
for script in test_ping.sh test_agents.sh; do
    LOG="$TESTS_DIR/${script%.sh}_${TIMESTAMP}.log"
    echo "Running $script -> $LOG"
    if ! bash "$TESTS_DIR/$script" > "$LOG" 2>&1; then
        echo "  FAILED (see $LOG)"
        FAILED=1
    else
        echo "  done"
    fi
done

exit $FAILED
