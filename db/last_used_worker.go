/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see LICENSE file for details.                                       *
 ******************************************************************************/

package db

import (
	"encoding/json"
	"time"

	"github.com/PivotLLM/MCPFusion/db/internal"
	"go.etcd.io/bbolt"
)

// lastUsedWorker is the single background goroutine responsible for persisting
// token last-used timestamps. It reads token hashes from lastUsedCh,
// deduplicates them in memory, and writes the batch to BoltDB either when
// LastUsedMaxBatch unique hashes have accumulated or every LastUsedFlushInterval,
// whichever comes first.
//
// On shutdown (stopLastUsed closed) it drains any remaining items and performs
// a final flush before returning.
func (d *DB) lastUsedWorker() {
	defer d.lastUsedWg.Done()

	ticker := time.NewTicker(internal.LastUsedFlushInterval)
	defer ticker.Stop()

	pending := make(map[string]struct{}, internal.LastUsedMaxBatch)

	flush := func() {
		if len(pending) == 0 {
			return
		}

		hashes := make([]string, 0, len(pending))
		for h := range pending {
			hashes = append(hashes, h)
		}
		pending = make(map[string]struct{}, internal.LastUsedMaxBatch)

		err := d.db.Update(func(tx *bbolt.Tx) error {
			tokensBucket := tx.Bucket([]byte(internal.BucketAPITokens))
			if tokensBucket == nil {
				return nil // bucket missing — nothing to update
			}
			now := time.Now()
			for _, hash := range hashes {
				metadataBytes := tokensBucket.Get([]byte(hash))
				if metadataBytes == nil {
					continue // token deleted — skip
				}
				var md APITokenMetadata
				if err := json.Unmarshal(metadataBytes, &md); err != nil {
					continue
				}
				md.LastUsed = now
				updated, err := json.Marshal(&md)
				if err != nil {
					continue
				}
				_ = tokensBucket.Put([]byte(hash), updated)
			}
			return nil
		})
		if err != nil {
			d.logger.Warningf("Failed to batch-update token last-used timestamps: %v", err)
		}
	}

	for {
		select {
		case hash := <-d.lastUsedCh:
			pending[hash] = struct{}{}
			if len(pending) >= internal.LastUsedMaxBatch {
				flush()
			}

		case <-ticker.C:
			flush()

		case <-d.stopLastUsed:
			// Drain any hashes already in the channel buffer before the final flush.
			for {
				select {
				case hash := <-d.lastUsedCh:
					pending[hash] = struct{}{}
				default:
					flush()
					return
				}
			}
		}
	}
}
