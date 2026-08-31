package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/PretendoNetwork/nex-go/v2/types"
	"github.com/PretendoNetwork/nex-protocols-go/v2/ranking/constants"
	ranking_types "github.com/PretendoNetwork/nex-protocols-go/v2/ranking/types"

	"github.com/lib/pq"

	"lost-reavers/globals"
)

// maxRankingLength is the largest page the RankingOrderParam.Length byte can
// express. A client asking for 0 entries would make the common protocol answer
// NotFound, which reads to the player as "no leaderboard", so treat 0 as "all
// you can fit".
const maxRankingLength = 255

// InsertRankingByPIDAndRankingScoreData stores a score uploaded through
// Ranking::UploadScore.
func InsertRankingByPIDAndRankingScoreData(pid types.PID, rankingScoreData ranking_types.RankingScoreData, uniqueID types.UInt64) error {
	orderBy, established := storedCategoryOrderBy(rankingScoreData.Category)
	if !established {
		orderBy = rankingScoreData.OrderBy
	}

	comparison := "<"
	if orderBy == constants.OrderByDescending {
		comparison = ">"
	}

	condition := fmt.Sprintf("EXCLUDED.score %s ranking.scores.score", comparison)
	if rankingScoreData.UpdateMode == constants.UpdateModeDeleteOld {
		condition = "true"
	}

	query := fmt.Sprintf(`
		INSERT INTO ranking.scores (pid, unique_id, category, score, order_by, groups, param, update_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7, (now() at time zone 'utc'))
		ON CONFLICT (pid, unique_id, category) DO UPDATE SET
			score       = EXCLUDED.score,
			groups      = EXCLUDED.groups,
			param       = EXCLUDED.param,
			update_time = EXCLUDED.update_time
		WHERE %s`, condition)

	_, err := Postgres.Exec(query,
		uint64(pid),
		uint64(uniqueID),
		uint32(rankingScoreData.Category),
		uint32(rankingScoreData.Score),
		uint8(orderBy),
		[]byte(rankingScoreData.Groups),
		uint64(rankingScoreData.Param),
	)

	return err
}

func storedCategoryOrderBy(category types.UInt32) (constants.OrderBy, bool) {
	var orderBy uint8

	err := Postgres.QueryRow(`SELECT order_by FROM ranking.scores WHERE category = $1 LIMIT 1`, uint32(category)).Scan(&orderBy)
	if errors.Is(err, sql.ErrNoRows) {
		return constants.OrderByDescending, false
	}

	if err != nil {
		globals.Logger.Errorf("Could not read the sort direction of category %d: %v", category, err)
		return constants.OrderByDescending, false
	}

	return constants.OrderBy(orderBy), true
}

func categoryOrderBy(category types.UInt32) constants.OrderBy {
	orderBy, _ := storedCategoryOrderBy(category)

	return orderBy
}

func scoreDirection(orderBy constants.OrderBy) string {
	if orderBy == constants.OrderByAscending {
		return "ASC"
	}

	return "DESC"
}

func tieBreakOrder(orderBy constants.OrderBy) string {
	return fmt.Sprintf("ORDER BY score %s, update_time ASC, pid ASC", scoreDirection(orderBy))
}

func scoreOnlyOrder(orderBy constants.OrderBy) string {
	return fmt.Sprintf("ORDER BY score %s", scoreDirection(orderBy))
}

func groupFilter(orderParam ranking_types.RankingOrderParam, args *[]any) string {
	if orderParam.GroupIndex == constants.FilterGroupIndexNone {
		return ""
	}

	*args = append(*args, int(orderParam.GroupIndex), int(orderParam.GroupNum))

	indexPlaceholder := len(*args) - 1
	valuePlaceholder := len(*args)

	return fmt.Sprintf(" AND octet_length(groups) > $%d AND get_byte(groups, $%d) = $%d",
		indexPlaceholder, indexPlaceholder, valuePlaceholder)
}

func pageLength(orderParam ranking_types.RankingOrderParam) int {
	length := int(orderParam.Length)
	if length == 0 || length > maxRankingLength {
		length = maxRankingLength
	}

	return length
}

type rankingQuery struct {
	category   types.UInt32
	orderParam ranking_types.RankingOrderParam
	scopePIDs  []uint64
	rowPID     *uint64
	centreOn   *uint64

	ignoreOffset bool
}

func (q rankingQuery) run() (types.List[ranking_types.RankingRankData], uint32, error) {
	rankings := types.NewList[ranking_types.RankingRankData]()

	orderBy := categoryOrderBy(q.category)

	args := []any{uint32(q.category)}

	scopeWhere := "category = $1" + groupFilter(q.orderParam, &args)

	if q.scopePIDs != nil {
		args = append(args, pq.Array(q.scopePIDs))
		scopeWhere += fmt.Sprintf(" AND pid = ANY($%d)", len(args))
	}

	length := pageLength(q.orderParam)
	offset := 0

	switch {
	case q.centreOn != nil:
		offset = centredOffset(scopeWhere, args, orderBy, *q.centreOn, length)
	case !q.ignoreOffset:
		offset = int(q.orderParam.Offset)
	}

	rowsWhere := ""
	if q.rowPID != nil {
		args = append(args, *q.rowPID)
		rowsWhere = fmt.Sprintf(" WHERE pid = $%d", len(args))
	}

	position := "RANK()"
	positionOrder := scoreOnlyOrder(orderBy)

	if q.orderParam.OrderCalculation == constants.OrderCalculation123 {
		position = "ROW_NUMBER()"
		positionOrder = tieBreakOrder(orderBy)
	}

	ordering := tieBreakOrder(orderBy)

	countQuery := fmt.Sprintf(
		`SELECT COUNT(*) FROM (SELECT pid FROM ranking.scores WHERE %s) AS scoped%s`,
		scopeWhere, rowsWhere)

	var totalCount uint32
	if err := Postgres.QueryRow(countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	if totalCount == 0 {
		return rankings, 0, nil
	}

	args = append(args, length, offset)

	query := fmt.Sprintf(`
		SELECT pid, unique_id, score, groups, param, update_time, position
		FROM (
			SELECT pid, unique_id, score, groups, param, update_time,
			       %s OVER (%s) AS position
			FROM ranking.scores WHERE %s
		) AS scoped%s
		%s LIMIT $%d OFFSET $%d`,
		position, positionOrder, scopeWhere, rowsWhere, ordering, len(args)-1, len(args))

	rows, err := Postgres.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			pid        uint64
			uniqueID   uint64
			score      uint32
			groups     []byte
			param      uint64
			updateTime types.DateTime
			position   uint32
		)

		if err := rows.Scan(&pid, &uniqueID, &score, &groups, &param, &updateTime, &position); err != nil {
			return nil, 0, err
		}

		rankData := ranking_types.NewRankingRankData()
		rankData.PrincipalID = types.NewPID(pid)
		rankData.UniqueID = types.NewUInt64(uniqueID)
		rankData.Category = q.category
		rankData.Score = types.NewUInt32(score)
		rankData.Groups = types.NewBuffer(groups)
		rankData.Param = types.NewUInt64(param)
		rankData.UpdateTime = updateTime
		rankData.CommonData = types.NewBuffer(commonDataFor(uniqueID, pid))
		rankData.Order = types.NewUInt32(position)

		rankings = append(rankings, rankData)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return rankings, totalCount, nil
}

func centredOffset(scopeWhere string, args []any, orderBy constants.OrderBy, pid uint64, length int) int {
	query := fmt.Sprintf(`
		SELECT position FROM (
			SELECT pid, ROW_NUMBER() OVER (%s) AS position
			FROM ranking.scores WHERE %s
		) AS scoped WHERE pid = $%d ORDER BY position ASC LIMIT 1`,
		tieBreakOrder(orderBy), scopeWhere, len(args)+1)

	var position int

	err := Postgres.QueryRow(query, append(append([]any{}, args...), pid)...).Scan(&position)
	if errors.Is(err, sql.ErrNoRows) {
		return 0
	}

	if err != nil {
		globals.Logger.Errorf("Could not locate PID %d on the board: %v", pid, err)
		return 0
	}

	offset := (position - 1) - length/2
	if offset < 0 {
		offset = 0
	}

	return offset
}

func commonDataFor(uniqueID uint64, pid uint64) []byte {
	var data []byte

	err := Postgres.QueryRow(`SELECT data FROM ranking.common_data WHERE unique_id = $1 AND pid = $2`, uniqueID, pid).Scan(&data)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		globals.Logger.Errorf("Could not read common data for unique ID %d (PID %d): %v", uniqueID, pid, err)
	}

	return data
}

func GetRankingsAndCountByCategoryAndRankingOrderParam(category types.UInt32, orderParam ranking_types.RankingOrderParam) (types.List[ranking_types.RankingRankData], uint32, error) {
	return rankingQuery{category: category, orderParam: orderParam}.run()
}

func GetOwnRankingByCategoryAndRankingOrderParam(pid types.PID, category types.UInt32, orderParam ranking_types.RankingOrderParam) (types.List[ranking_types.RankingRankData], uint32, error) {
	own := uint64(pid)

	return rankingQuery{
		category:     category,
		orderParam:   orderParam,
		rowPID:       &own,
		ignoreOffset: true,
	}.run()
}

func GetFriendsRankingsAndCountByCategoryAndRankingOrderParam(pid types.PID, category types.UInt32, orderParam ranking_types.RankingOrderParam) (types.List[ranking_types.RankingRankData], uint32, error) {
	return rankingQuery{category: category, orderParam: orderParam, scopePIDs: friendCircle(pid)}.run()
}

func GetNearbyRankingsAndCountByCategoryAndRankingOrderParam(pid types.PID, category types.UInt32, orderParam ranking_types.RankingOrderParam) (types.List[ranking_types.RankingRankData], uint32, error) {
	centre := uint64(pid)

	return rankingQuery{category: category, orderParam: orderParam, centreOn: &centre}.run()
}

func GetNearbyFriendsRankingsAndCountByCategoryAndRankingOrderParam(pid types.PID, category types.UInt32, orderParam ranking_types.RankingOrderParam) (types.List[ranking_types.RankingRankData], uint32, error) {
	centre := uint64(pid)

	return rankingQuery{
		category:   category,
		orderParam: orderParam,
		scopePIDs:  friendCircle(pid),
		centreOn:   &centre,
	}.run()
}

func friendCircle(pid types.PID) []uint64 {
	circle := []uint64{uint64(pid)}

	for _, friend := range globals.GetUserFriendPIDs(uint32(pid)) {
		circle = append(circle, uint64(friend))
	}

	return circle
}

func GetCommonData(uniqueID types.UInt64) (types.Buffer, error) {
	if uniqueID == 0 {
		return nil, errors.New("common data cannot be looked up by unique ID 0")
	}

	var data []byte

	err := Postgres.QueryRow(`SELECT data FROM ranking.common_data WHERE unique_id = $1 ORDER BY pid ASC LIMIT 1`, uint64(uniqueID)).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("no common data stored for unique ID %d", uint64(uniqueID))
	}

	if err != nil {
		return nil, err
	}

	return types.NewBuffer(data), nil
}

func UploadCommonData(pid types.PID, uniqueID types.UInt64, commonData types.Buffer) error {
	if len(commonData) > constants.MaxCommonDataSize {
		return fmt.Errorf("common data is %d bytes, the protocol allows at most %d", len(commonData), constants.MaxCommonDataSize)
	}

	_, err := Postgres.Exec(`
		INSERT INTO ranking.common_data (unique_id, pid, data)
		VALUES ($1, $2, $3)
		ON CONFLICT (unique_id, pid) DO UPDATE SET data = EXCLUDED.data`,
		uint64(uniqueID), uint64(pid), []byte(commonData))

	return err
}
