package service

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yaourtv/sapling-api/internal/db/query"
)

// RunAutoArchive periodically moves stale applications to the user's system
// ("Lost") status. Runs until ctx is cancelled.
func RunAutoArchive(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := archiveStaleApplications(ctx, pool); err != nil {
				log.Printf("auto-archive: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// archiveStaleApplications archives applications whose updated_at is older
// than the owner's archive_after_days (0 disables) and inserts a history row
// for each archived application.
func archiveStaleApplications(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := query.New(pool).WithTx(tx)
	archivedIDs, err := qtx.ArchiveStaleApplications(ctx)
	if err != nil {
		return err
	}
	if len(archivedIDs) == 0 {
		return tx.Commit(ctx)
	}
	if err := qtx.InsertArchiveHistory(ctx, archivedIDs); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	log.Printf("auto-archive: archived %d stale applications", len(archivedIDs))
	return nil
}
