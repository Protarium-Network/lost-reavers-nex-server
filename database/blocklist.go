package database

import (
	"github.com/PretendoNetwork/nex-go/v2/types"

	"github.com/lib/pq"

	"lost-reavers/globals"
)

// The block list is per-player and directional in storage but enforced both
// ways: if either side has blocked the other, they are kept out of the same
// party.

func GetBlockList(pid types.PID) (types.List[types.PID], error) {
	blocked := types.NewList[types.PID]()

	rows, err := Postgres.Query(`SELECT blocked_pid FROM matchmaking.block_list WHERE pid = $1 ORDER BY blocked_pid`, uint64(pid))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var blockedPID uint64

		if err := rows.Scan(&blockedPID); err != nil {
			return nil, err
		}

		blocked = append(blocked, types.NewPID(blockedPID))
	}

	return blocked, rows.Err()
}

func AddToBlockList(pid types.PID, blocked types.List[types.PID]) error {
	for _, blockedPID := range blocked {
		if blockedPID.Equals(pid) {
			continue
		}

		_, err := Postgres.Exec(`
			INSERT INTO matchmaking.block_list (pid, blocked_pid) VALUES ($1, $2)
			ON CONFLICT (pid, blocked_pid) DO NOTHING`,
			uint64(pid), uint64(blockedPID))
		if err != nil {
			return err
		}
	}

	return nil
}

func RemoveFromBlockList(pid types.PID, unblocked types.List[types.PID]) error {
	pids := make([]uint64, 0, len(unblocked))
	for _, unblockedPID := range unblocked {
		pids = append(pids, uint64(unblockedPID))
	}

	if len(pids) == 0 {
		return nil
	}

	_, err := Postgres.Exec(`DELETE FROM matchmaking.block_list WHERE pid = $1 AND blocked_pid = ANY($2)`,
		uint64(pid), pq.Array(pids))

	return err
}

func ClearBlockList(pid types.PID) error {
	_, err := Postgres.Exec(`DELETE FROM matchmaking.block_list WHERE pid = $1`, uint64(pid))

	return err
}

func IsBlockedEitherWay(a types.PID, b types.PID) bool {
	var blocked bool

	err := Postgres.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM matchmaking.block_list
			WHERE (pid = $1 AND blocked_pid = $2) OR (pid = $2 AND blocked_pid = $1)
		)`, uint64(a), uint64(b)).Scan(&blocked)
	if err != nil {
		globals.Logger.Errorf("Could not check the block list for PIDs %d and %d, allowing the party: %v", uint64(a), uint64(b), err)
		return false
	}

	return blocked
}
