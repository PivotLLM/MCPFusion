/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package internal

import "time"

// MaxKnowledgeQueryLength is the maximum number of characters accepted in a
// knowledge search query.  Queries beyond this length offer no practical benefit
// and impose unbounded CPU cost on the BoltDB scan.
const MaxKnowledgeQueryLength = 512

// LastUsedChannelSize is the number of token-hash slots buffered in the
// last-used update channel. When the buffer is full, new updates are
// silently dropped — the timestamp is best-effort.
const LastUsedChannelSize = 512

// LastUsedMaxBatch is the maximum number of unique token hashes accumulated
// before an early flush is triggered, independent of the flush interval.
const LastUsedMaxBatch = 64

// LastUsedFlushInterval is how often the worker flushes pending token
// last-used timestamps to BoltDB in a single batched write transaction.
const LastUsedFlushInterval = 30 * time.Second
