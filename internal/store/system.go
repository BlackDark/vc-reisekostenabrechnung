package store

import "context"

// SizeBytes reports the SQLite database size (page count * page size) read from
// the read pool. It backs the admin system status.
func (s *Store) SizeBytes(ctx context.Context) (int64, error) {
	var pages, pageSize int64
	if err := s.read.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pages); err != nil {
		return 0, err
	}
	if err := s.read.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, err
	}
	return pages * pageSize, nil
}

// JobCounts reports how many jobs are open (pending or running), done, and
// failed. It backs the admin system status.
func (s *Store) JobCounts(ctx context.Context) (offen, erledigt, fehl int64, err error) {
	if offen, err = s.readQ().CountJobsOpen(ctx); err != nil {
		return 0, 0, 0, err
	}
	if erledigt, err = s.readQ().CountJobsDone(ctx); err != nil {
		return 0, 0, 0, err
	}
	if fehl, err = s.readQ().CountJobsFailed(ctx); err != nil {
		return 0, 0, 0, err
	}
	return offen, erledigt, fehl, nil
}
