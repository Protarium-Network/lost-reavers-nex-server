package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/PretendoNetwork/nex-go/v2"
	"github.com/PretendoNetwork/nex-go/v2/types"
	datastore_types "github.com/PretendoNetwork/nex-protocols-go/v2/datastore/types"
	"github.com/lib/pq"

	"lost-reavers/globals"
)

// initDataStoreSchema creates the tables DataStore needs. Object bodies live
// in S3 (see database/s3.go); everything the console can search or filter on
// lives here instead, the same split nex-protocols-common-go's S3Presigner
// expects.
func initDataStoreSchema() error {
	statements := []string{
		`CREATE SCHEMA IF NOT EXISTS datastore`,

		`CREATE SEQUENCE IF NOT EXISTS datastore.object_data_id_seq
			INCREMENT 1 MINVALUE 1 MAXVALUE 281474976710656 START 1 CACHE 1`,

		`CREATE TABLE IF NOT EXISTS datastore.objects (
			data_id                      bigint NOT NULL DEFAULT nextval('datastore.object_data_id_seq') PRIMARY KEY,
			upload_completed             boolean NOT NULL DEFAULT FALSE,
			deleted                      boolean NOT NULL DEFAULT FALSE,
			owner                        bigint,
			size                         int,
			name                         text,
			data_type                    int,
			meta_binary                  bytea,
			permission                   int,
			permission_recipients        bigint[],
			delete_permission            int,
			delete_permission_recipients bigint[],
			flag                         int,
			period                       int,
			refer_data_id                bigint,
			tags                         text[],
			persistence_slot_id          int,
			extra_data                   text[],
			access_password              bigint NOT NULL DEFAULT 0,
			update_password              bigint NOT NULL DEFAULT 0,
			creation_date                timestamp,
			update_date                  timestamp
		)`,

		`CREATE INDEX IF NOT EXISTS objects_owner_idx ON datastore.objects (owner) WHERE NOT deleted`,
		`CREATE INDEX IF NOT EXISTS objects_persistence_idx ON datastore.objects (owner, persistence_slot_id) WHERE NOT deleted`,

		// * One row per (data_id, slot): the running total and count a rating
		// * average is computed from. RateObjectWithPassword's signature (see
		// * below) carries no caller PID, matching the real protocol -- NEX
		// * ratings are a plain accumulator with no per-player dedup at this
		// * layer, so repeat calls from the same console add again, exactly as
		// * a retail server does.
		`CREATE TABLE IF NOT EXISTS datastore.ratings (
			data_id       bigint NOT NULL,
			slot          smallint NOT NULL,
			total_value   bigint NOT NULL DEFAULT 0,
			count         bigint NOT NULL DEFAULT 0,
			initial_value bigint NOT NULL DEFAULT 0,
			PRIMARY KEY (data_id, slot)
		)`,
	}

	for _, statement := range statements {
		if _, err := Postgres.Exec(statement); err != nil {
			return fmt.Errorf("failed to create datastore tables: %w", err)
		}
	}

	globals.Logger.Success("datastore Postgres schema ready")

	return nil
}

func convertPIDList(list *types.List[types.PID]) []uint64 {
	result := make([]uint64, len(*list))
	for i, pid := range *list {
		result[i] = uint64(pid)
	}
	return result
}

func createPIDList(list *[]uint64) types.List[types.PID] {
	result := make([]types.PID, len(*list))
	for i, u := range *list {
		result[i] = types.NewPID(u)
	}
	return result
}

func convertStringList(list *types.List[types.String]) []string {
	result := make([]string, len(*list))
	for i, n := range *list {
		result[i] = n.String()
	}
	return result
}

func createStringList(list *[]string) types.List[types.String] {
	result := make([]types.String, len(*list))
	for i, u := range *list {
		result[i] = types.NewString(u)
	}
	return result
}

// dataStoreColumns is the column list every SELECT against datastore.objects
// uses, so parseObjectList's Scan order can never drift from a `SELECT *`
// that silently reordered after a migration.
const dataStoreColumns = `data_id, upload_completed, deleted, owner, size, name, data_type, meta_binary,
	permission, permission_recipients, delete_permission, delete_permission_recipients, flag, period,
	refer_data_id, tags, persistence_slot_id, extra_data, access_password, update_password, creation_date, update_date`

func parseObjectList(rows *sql.Rows) ([]datastore_types.DataStoreMetaInfo, error) {
	var results []datastore_types.DataStoreMetaInfo

	for rows.Next() {
		result := datastore_types.NewDataStoreMetaInfo()

		var (
			uploadCompleted         bool
			deleted                 bool
			ownerID                 uint64
			permissionRecipients    []int64
			delPermissionRecipients []int64
			tags                    []string
			extraData               []string
			persistenceSlotID       int64
			createdTime             time.Time
			updatedTime             time.Time
		)

		// * 9999-12-31T00:00:00.000Z -- what the real service sends for objects
		// * that never expire.
		result.ExpireTime = types.NewDateTime(0x9C3f3E0000)

		err := rows.Scan(
			&result.DataID,
			&uploadCompleted,
			&deleted,
			&ownerID,
			&result.Size,
			&result.Name,
			&result.DataType,
			&result.MetaBinary,
			&result.Permission.Permission,
			pq.Array(&permissionRecipients),
			&result.DelPermission.Permission,
			pq.Array(&delPermissionRecipients),
			&result.Flag,
			&result.Period,
			&result.ReferDataID,
			pq.Array(&tags),
			&persistenceSlotID,
			pq.Array(&extraData),
			new(int64), // access_password: never returned to the client directly
			new(int64), // update_password: ditto
			&createdTime,
			&updatedTime,
		)
		if err != nil {
			return nil, err
		}

		if deleted {
			continue
		}

		permissionRecipientsU := make([]uint64, len(permissionRecipients))
		for i, v := range permissionRecipients {
			permissionRecipientsU[i] = uint64(v)
		}

		delPermissionRecipientsU := make([]uint64, len(delPermissionRecipients))
		for i, v := range delPermissionRecipients {
			delPermissionRecipientsU[i] = uint64(v)
		}

		result.OwnerID = types.NewPID(ownerID)
		result.Permission.RecipientIDs = createPIDList(&permissionRecipientsU)
		result.DelPermission.RecipientIDs = createPIDList(&delPermissionRecipientsU)
		result.Tags = createStringList(&tags)
		result.CreatedTime.FromTimestamp(createdTime)
		result.UpdatedTime.FromTimestamp(updatedTime)
		result.ReferredTime.FromTimestamp(createdTime)

		results = append(results, result)
	}

	return results, rows.Err()
}

// InitializeObjectByPreparePostParam backs DataStore::PreparePostObject: the
// console has decided to upload something (a treasure haul, an inventory
// snapshot, ...) and this reserves the metadata row and data ID before the
// actual bytes go to S3.
func InitializeObjectByPreparePostParam(ownerPID types.PID, param datastore_types.DataStorePreparePostParam) (uint64, *nex.Error) {
	now := time.Now().UTC()

	var dataID uint64

	err := Postgres.QueryRow(`
		INSERT INTO datastore.objects (
			upload_completed, deleted, owner, size, name, data_type, meta_binary,
			permission, permission_recipients, delete_permission, delete_permission_recipients,
			flag, period, refer_data_id, tags, persistence_slot_id, extra_data,
			access_password, update_password, creation_date, update_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		RETURNING data_id`,
		false, false, uint64(ownerPID), uint32(param.Size), string(param.Name), uint16(param.DataType), []byte(param.MetaBinary),
		int(param.Permission.Permission), pq.Array(convertPIDList(&param.Permission.RecipientIDs)),
		int(param.DelPermission.Permission), pq.Array(convertPIDList(&param.DelPermission.RecipientIDs)),
		uint32(param.Flag), uint16(param.Period), uint64(param.ReferDataID),
		pq.Array(convertStringList(&param.Tags)), int(param.PersistenceInitParam.PersistenceSlotID),
		pq.Array(convertStringList(&param.ExtraData)), 0, 0, now, now,
	).Scan(&dataID)
	if err != nil {
		globals.Logger.Errorf("InitializeObjectByPreparePostParam: %v", err)
		return 0, nex.NewError(nex.ResultCodes.DataStore.Unknown, "change_error")
	}

	return dataID, nil
}

// InitializeObjectRatingWithSlot seeds the aggregate row a rating slot needs
// before RateObjectWithPassword can accumulate into it.
func InitializeObjectRatingWithSlot(dataID uint64, param datastore_types.DataStoreRatingInitParamWithSlot) *nex.Error {
	_, err := Postgres.Exec(`
		INSERT INTO datastore.ratings (data_id, slot, total_value, count, initial_value)
		VALUES ($1, $2, 0, 0, $3)
		ON CONFLICT (data_id, slot) DO UPDATE SET initial_value = EXCLUDED.initial_value`,
		int64(dataID), int8(param.Slot), int64(param.Param.InitialValue))
	if err != nil {
		globals.Logger.Errorf("InitializeObjectRatingWithSlot: %v", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "change_error")
	}

	return nil
}

// GetObjectInfoByDataID backs GetMeta/PrepareGetObject when the console
// addresses an object directly.
func GetObjectInfoByDataID(dataID types.UInt64) (datastore_types.DataStoreMetaInfo, *nex.Error) {
	rows, err := Postgres.Query(`SELECT `+dataStoreColumns+` FROM datastore.objects WHERE data_id = $1 AND NOT deleted`, int64(dataID))
	if err != nil {
		globals.Logger.Errorf("GetObjectInfoByDataID: %v", err)
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}
	defer rows.Close()

	results, err := parseObjectList(rows)
	if err != nil || len(results) == 0 {
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}

	return results[0], nil
}

// GetObjectInfoByDataIDWithPassword is GetObjectInfoByDataID plus the access
// password check PrepareGetObject makes when an object requires one.
func GetObjectInfoByDataIDWithPassword(dataID types.UInt64, password types.UInt64) (datastore_types.DataStoreMetaInfo, *nex.Error) {
	info, nexErr := GetObjectInfoByDataID(dataID)
	if nexErr != nil {
		return info, nexErr
	}

	var storedPassword uint64
	if err := Postgres.QueryRow(`SELECT access_password FROM datastore.objects WHERE data_id = $1`, int64(dataID)).Scan(&storedPassword); err != nil {
		globals.Logger.Errorf("GetObjectInfoByDataIDWithPassword: %v", err)
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}

	if storedPassword != 0 && storedPassword != uint64(password) {
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.PermissionDenied, "change_error")
	}

	return info, nil
}

// GetObjectInfoByPersistenceTargetWithPassword resolves "the object in this
// player's persistence slot N" -- e.g. a character's single save slot -- which
// is how PrepareGetObject is called when the console has no data ID yet.
func GetObjectInfoByPersistenceTargetWithPassword(target datastore_types.DataStorePersistenceTarget, password types.UInt64) (datastore_types.DataStoreMetaInfo, *nex.Error) {
	var dataID uint64

	err := Postgres.QueryRow(`
		SELECT data_id FROM datastore.objects
		WHERE owner = $1 AND persistence_slot_id = $2 AND NOT deleted
		ORDER BY creation_date DESC LIMIT 1`,
		uint64(target.OwnerID), int(target.PersistenceSlotID),
	).Scan(&dataID)
	if errors.Is(err, sql.ErrNoRows) {
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}
	if err != nil {
		globals.Logger.Errorf("GetObjectInfoByPersistenceTargetWithPassword: %v", err)
		return datastore_types.DataStoreMetaInfo{}, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}

	return GetObjectInfoByDataIDWithPassword(types.NewUInt64(dataID), password)
}

func GetObjectSizeByDataID(dataID types.UInt64) (uint32, *nex.Error) {
	var size uint32

	err := Postgres.QueryRow(`SELECT size FROM datastore.objects WHERE data_id = $1 AND NOT deleted`, int64(dataID)).Scan(&size)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}
	if err != nil {
		globals.Logger.Errorf("GetObjectSizeByDataID: %v", err)
		return 0, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}

	return size, nil
}

func GetObjectOwnerByDataID(dataID types.UInt64) (uint32, *nex.Error) {
	var owner uint64

	err := Postgres.QueryRow(`SELECT owner FROM datastore.objects WHERE data_id = $1 AND NOT deleted`, int64(dataID)).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}
	if err != nil {
		globals.Logger.Errorf("GetObjectOwnerByDataID: %v", err)
		return 0, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}

	return uint32(owner), nil
}

// UpdateObjectUploadCompletedByDataID backs CompletePostObject: the console
// finished the S3 upload PreparePostObject presigned, and the object becomes
// visible to searches and GetMeta from here on.
func UpdateObjectUploadCompletedByDataID(dataID types.UInt64, uploadCompleted bool) *nex.Error {
	_, err := Postgres.Exec(`UPDATE datastore.objects SET upload_completed = $1, update_date = $2 WHERE data_id = $3`,
		uploadCompleted, time.Now().UTC(), int64(dataID))
	if err != nil {
		globals.Logger.Errorf("UpdateObjectUploadCompletedByDataID: %v", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "change_error")
	}

	return nil
}

func UpdateObjectPeriodByDataIDWithPassword(dataID types.UInt64, period types.UInt16, updatePassword types.UInt64) *nex.Error {
	_, err := Postgres.Exec(`UPDATE datastore.objects SET period = $1, update_date = $2 WHERE data_id = $3`,
		uint16(period), time.Now().UTC(), int64(dataID))
	if err != nil {
		globals.Logger.Errorf("UpdateObjectPeriodByDataIDWithPassword: %v", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "change_error")
	}

	return nil
}

func UpdateObjectMetaBinaryByDataIDWithPassword(dataID types.UInt64, metaBinary types.QBuffer, updatePassword types.UInt64) *nex.Error {
	_, err := Postgres.Exec(`UPDATE datastore.objects SET meta_binary = $1, update_date = $2 WHERE data_id = $3`,
		[]byte(metaBinary), time.Now().UTC(), int64(dataID))
	if err != nil {
		globals.Logger.Errorf("UpdateObjectMetaBinaryByDataIDWithPassword: %v", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "change_error")
	}

	return nil
}

func UpdateObjectDataTypeByDataIDWithPassword(dataID types.UInt64, dataType types.UInt16, updatePassword types.UInt64) *nex.Error {
	_, err := Postgres.Exec(`UPDATE datastore.objects SET data_type = $1, update_date = $2 WHERE data_id = $3`,
		uint16(dataType), time.Now().UTC(), int64(dataID))
	if err != nil {
		globals.Logger.Errorf("UpdateObjectDataTypeByDataIDWithPassword: %v", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "change_error")
	}

	return nil
}

// DeleteObjectByDataID backs Utility-side cleanup and DeleteObject calls that
// carry no password (the caller was already verified as the owner upstream).
func DeleteObjectByDataID(dataID types.UInt64) *nex.Error {
	_, err := Postgres.Exec(`UPDATE datastore.objects SET deleted = true, update_date = $1 WHERE data_id = $2`,
		time.Now().UTC(), int64(dataID))
	if err != nil {
		globals.Logger.Errorf("DeleteObjectByDataID: %v", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "change_error")
	}

	return nil
}

func DeleteObjectByDataIDWithPassword(dataID types.UInt64, password types.UInt64) *nex.Error {
	var storedPassword uint64

	err := Postgres.QueryRow(`SELECT access_password FROM datastore.objects WHERE data_id = $1 AND NOT deleted`, int64(dataID)).Scan(&storedPassword)
	if errors.Is(err, sql.ErrNoRows) {
		return nex.NewError(nex.ResultCodes.DataStore.NotFound, "not found")
	}
	if err != nil {
		globals.Logger.Errorf("DeleteObjectByDataIDWithPassword: %v", err)
		return nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}

	if storedPassword != 0 && storedPassword != uint64(password) {
		return nex.NewError(nex.ResultCodes.DataStore.PermissionDenied, "change_error")
	}

	return DeleteObjectByDataID(dataID)
}

// GetObjectInfosByDataStoreSearchParam backs DataStore::SearchObject: the
// title's "browse other players' shared objects" flow. Filtering here is
// intentionally conservative -- owner, tags and data type, the fields every
// Wii U title's search actually narrows on -- rather than every field the
// protocol defines, most of which nobody has observed Lost Reavers send yet
// (see docs/PROTOCOLS.md).
func GetObjectInfosByDataStoreSearchParam(param datastore_types.DataStoreSearchParam, pid types.PID) ([]datastore_types.DataStoreMetaInfo, uint32, *nex.Error) {
	query := `SELECT ` + dataStoreColumns + ` FROM datastore.objects WHERE NOT deleted AND upload_completed`
	var args []any

	if len(param.OwnerIDs) > 0 {
		args = append(args, pq.Array(convertPIDList(&param.OwnerIDs)))
		query += fmt.Sprintf(" AND owner = ANY($%d)", len(args))
	}

	if param.DataType != 0 {
		args = append(args, uint16(param.DataType))
		query += fmt.Sprintf(" AND data_type = $%d", len(args))
	}

	if len(param.Tags) > 0 {
		args = append(args, pq.Array(convertStringList(&param.Tags)))
		query += fmt.Sprintf(" AND tags && $%d", len(args))
	}

	limit := int(param.ResultRange.Length)
	if limit <= 0 || limit > 255 {
		limit = 255
	}

	args = append(args, limit, int(param.ResultRange.Offset))
	query += fmt.Sprintf(" ORDER BY creation_date DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := Postgres.Query(query, args...)
	if err != nil {
		globals.Logger.Errorf("GetObjectInfosByDataStoreSearchParam: %v", err)
		return nil, 0, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}
	defer rows.Close()

	results, err := parseObjectList(rows)
	if err != nil {
		globals.Logger.Errorf("GetObjectInfosByDataStoreSearchParam: %v", err)
		return nil, 0, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}

	return results, uint32(len(results)), nil
}

// RateObjectWithPassword backs DataStore::RateObject: accumulate one vote
// into a rating slot and return the new running total. The protocol's own
// signature carries no caller PID, so -- matching real NEX rating semantics --
// this is a plain accumulator with no per-console dedup; a lock type/period
// on the slot, if the title uses one, is enforced by the common protocol
// before this hook runs, not here.
func RateObjectWithPassword(dataID types.UInt64, slot types.UInt8, ratingValue types.Int32, accessPassword types.UInt64) (datastore_types.DataStoreRatingInfo, *nex.Error) {
	if _, err := Postgres.Exec(`
		INSERT INTO datastore.ratings (data_id, slot, total_value, count)
		VALUES ($1, $2, 0, 0) ON CONFLICT (data_id, slot) DO NOTHING`,
		int64(dataID), int8(slot)); err != nil {
		globals.Logger.Errorf("RateObjectWithPassword: %v", err)
		return datastore_types.DataStoreRatingInfo{}, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}

	var totalValue, count, initialValue int64

	err := Postgres.QueryRow(`
		UPDATE datastore.ratings SET total_value = total_value + $1, count = count + 1
		WHERE data_id = $2 AND slot = $3
		RETURNING total_value, count, initial_value`,
		int64(ratingValue), int64(dataID), int8(slot),
	).Scan(&totalValue, &count, &initialValue)
	if err != nil {
		globals.Logger.Errorf("RateObjectWithPassword: %v", err)
		return datastore_types.DataStoreRatingInfo{}, nex.NewError(nex.ResultCodes.DataStore.Unknown, "unknown")
	}

	var info datastore_types.DataStoreRatingInfo
	info.TotalValue = types.NewInt64(totalValue)
	info.Count = types.NewUInt32(uint32(count))
	info.InitialValue = types.NewInt64(initialValue)

	return info, nil
}
