package helpers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/internal/logger"
	"github.com/transferia/transferia/pkg/abstract"
	"github.com/transferia/transferia/pkg/providers/clickhouse"
	chModel "github.com/transferia/transferia/pkg/providers/clickhouse/model"
	mongoStorage "github.com/transferia/transferia/pkg/providers/mongo"
	mysqlStorage "github.com/transferia/transferia/pkg/providers/mysql"
	pgStorage "github.com/transferia/transferia/pkg/providers/postgres"
	"github.com/transferia/transferia/pkg/worker/tasks"
	"go.ytsaurus.tech/library/go/core/log"
)

var technicalTables = map[string]bool{
	"__data_transfer_signal_table": true, // dblog signal table
	"__consumer_keeper":            true, // pg
	"__dt_cluster_time":            true, // mongodb
	"__table_transfer_progress":    true, // mysql
	"__tm_gtid_keeper":             true, // mysql
	"__tm_keeper":                  true, // mysql
}

func withTextSerialization(storageParams *pgStorage.PgStorageParams) *pgStorage.PgStorageParams {
	// Checksum does not support comparing binary values for now. Use
	// the text types instead, even in homogeneous pg->pg transfers.
	storageParams.UseBinarySerialization = false
	return storageParams
}

func GetSampleableStorageByModel(t *testing.T, serverModel interface{}) abstract.ChecksumableStorage {
	var result abstract.ChecksumableStorage
	var err error

	switch model := serverModel.(type) {
	// pg
	case pgStorage.PgSource:
		result, err = pgStorage.NewStorage(withTextSerialization(model.ToStorageParams(nil)))
	case *pgStorage.PgSource:
		result, err = pgStorage.NewStorage(withTextSerialization(model.ToStorageParams(nil)))
	case pgStorage.PgDestination:
		result, err = pgStorage.NewStorage(withTextSerialization(model.ToStorageParams()))
	case *pgStorage.PgDestination:
		result, err = pgStorage.NewStorage(withTextSerialization(model.ToStorageParams()))
	// ch
	case chModel.ChSource:
		storageParams, storageParamsErr := model.ToStorageParams()
		require.NoError(t, storageParamsErr)
		result, err = clickhouse.NewStorage(storageParams, nil)
	case *chModel.ChSource:
		storageParams, storageParamsErr := model.ToStorageParams()
		require.NoError(t, storageParamsErr)
		result, err = clickhouse.NewStorage(storageParams, nil)
	case chModel.ChDestination:
		storageParams, storageParamsErr := model.ToStorageParams()
		require.NoError(t, storageParamsErr)
		result, err = clickhouse.NewStorage(storageParams, nil)
	case *chModel.ChDestination:
		storageParams, storageParamsErr := model.ToStorageParams()
		require.NoError(t, storageParamsErr)
		result, err = clickhouse.NewStorage(storageParams, nil)
	// mysql
	case mysqlStorage.MysqlSource:
		result, err = mysqlStorage.NewStorage(model.ToStorageParams())
	case *mysqlStorage.MysqlSource:
		result, err = mysqlStorage.NewStorage(model.ToStorageParams())
	case mysqlStorage.MysqlDestination:
		result, err = mysqlStorage.NewStorage(model.ToStorageParams())
	case *mysqlStorage.MysqlDestination:
		result, err = mysqlStorage.NewStorage(model.ToStorageParams())
	// mongo
	case mongoStorage.MongoSource:
		result, err = mongoStorage.NewStorage(model.ToStorageParams())
	case *mongoStorage.MongoSource:
		result, err = mongoStorage.NewStorage(model.ToStorageParams())
	case mongoStorage.MongoDestination:
		result, err = mongoStorage.NewStorage(model.ToStorageParams())
	case *mongoStorage.MongoDestination:
		result, err = mongoStorage.NewStorage(model.ToStorageParams())
	default:
		require.Fail(t, fmt.Sprintf("unknown type of serverModel: %T", serverModel))
	}

	if err != nil {
		require.Fail(t, fmt.Sprintf("unable to create storage: %s", err))
	}

	return result
}

func FilterTechnicalTables(tables abstract.TableMap) []abstract.TableDescription {
	result := make([]abstract.TableDescription, 0)
	for _, el := range tables.ConvertToTableDescriptions() {
		if technicalTables[el.Name] {
			continue
		}
		result = append(result, el)
	}
	return result
}

type CompareStoragesParams struct {
	EqualDataTypes      func(lDataType, rDataType string) bool
	TableFilter         func(tables abstract.TableMap) []abstract.TableDescription
	PriorityComparators []tasks.ChecksumComparator
	StableFallback      bool
	StableRowLimit      int
	DebugSampleRows     int
}

func NewCompareStorageParams() *CompareStoragesParams {
	return &CompareStoragesParams{
		EqualDataTypes:      StrictEquality,
		TableFilter:         FilterTechnicalTables,
		PriorityComparators: nil,
		StableFallback:      false,
		StableRowLimit:      10000,
		DebugSampleRows:     20,
	}
}

func (p *CompareStoragesParams) WithEqualDataTypes(equalDataTypes func(lDataType, rDataType string) bool) *CompareStoragesParams {
	p.EqualDataTypes = equalDataTypes
	return p
}

func (p *CompareStoragesParams) WithTableFilter(tableFilter func(tables abstract.TableMap) []abstract.TableDescription) *CompareStoragesParams {
	p.TableFilter = tableFilter
	return p
}

func (p *CompareStoragesParams) WithPriorityComparators(comparators ...tasks.ChecksumComparator) *CompareStoragesParams {
	p.PriorityComparators = comparators
	return p
}

func (p *CompareStoragesParams) WithStableFallback(enabled bool) *CompareStoragesParams {
	p.StableFallback = enabled
	return p
}

func (p *CompareStoragesParams) WithStableRowLimit(limit int) *CompareStoragesParams {
	p.StableRowLimit = limit
	return p
}

func (p *CompareStoragesParams) WithDebugSampleRows(limit int) *CompareStoragesParams {
	p.DebugSampleRows = limit
	return p
}

func CompareStorages(t *testing.T, sourceModel, targetModel interface{}, params *CompareStoragesParams) error {
	srcStorage := GetSampleableStorageByModel(t, sourceModel)
	dstStorage := GetSampleableStorageByModel(t, targetModel)
	switch src := srcStorage.(type) {
	case *mysqlStorage.Storage:
		dst, ok := dstStorage.(*mysqlStorage.Storage)
		if ok {
			src.IsHomo = true
			dst.IsHomo = true
		}
	}
	all, err := srcStorage.TableList(nil)
	require.NoError(t, err)
	checksumErr := tasks.CompareChecksum(
		srcStorage,
		dstStorage,
		params.TableFilter(all),
		logger.Log,
		EmptyRegistry(),
		params.EqualDataTypes,
		&tasks.ChecksumParameters{
			TableSizeThreshold:  0,
			Tables:              nil,
			PriorityComparators: params.PriorityComparators,
		},
	)
	return applyStableFallback(checksumErr, params, func() error {
		return compareStoragesStable(srcStorage, dstStorage, params, params.TableFilter(all))
	})
}

func applyStableFallback(checksumErr error, params *CompareStoragesParams, fallback func() error) error {
	if checksumErr == nil {
		return nil
	}
	if params == nil || !params.StableFallback {
		return checksumErr
	}
	if fallback == nil {
		return checksumErr
	}
	if err := fallback(); err != nil {
		return fmt.Errorf("%w; stable fallback failed: %w", checksumErr, err)
	}
	return nil
}

func compareStoragesStable(srcStorage, dstStorage abstract.Storage, params *CompareStoragesParams, tables []abstract.TableDescription) error {
	if params.StableRowLimit <= 0 {
		return fmt.Errorf("stable fallback requires StableRowLimit > 0")
	}
	if params.DebugSampleRows <= 0 {
		params.DebugSampleRows = 20
	}
	for _, table := range tables {
		srcRows, err := loadAllRows(srcStorage, table, params.StableRowLimit)
		if err != nil {
			return fmt.Errorf("load source rows for %s: %w", table.ID().Fqtn(), err)
		}
		dstRows, err := loadAllRows(dstStorage, table, params.StableRowLimit)
		if err != nil {
			return fmt.Errorf("load target rows for %s: %w", table.ID().Fqtn(), err)
		}
		mismatches, samples, err := compareLoadedRowsStable(table, srcRows, dstRows, params.PriorityComparators, params.DebugSampleRows)
		if err != nil {
			return err
		}
		if mismatches > 0 {
			return fmt.Errorf("stable compare mismatch for %s: %d mismatch(es): %s", table.ID().Fqtn(), mismatches, strings.Join(samples, "; "))
		}
	}
	return nil
}

func loadAllRows(storage abstract.Storage, table abstract.TableDescription, stableRowLimit int) ([]abstract.ChangeItem, error) {
	rows := make([]abstract.ChangeItem, 0)
	err := storage.LoadTable(context.Background(), table, func(input []abstract.ChangeItem) error {
		rows = append(rows, input...)
		if len(rows) > stableRowLimit {
			return fmt.Errorf("row count exceeds StableRowLimit=%d", stableRowLimit)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func compareLoadedRowsStable(
	table abstract.TableDescription,
	srcRows []abstract.ChangeItem,
	dstRows []abstract.ChangeItem,
	priorityComparators []tasks.ChecksumComparator,
	debugSampleRows int,
) (int, []string, error) {
	if len(srcRows) != len(dstRows) {
		return 1, []string{fmt.Sprintf("%s row_count src=%d dst=%d", table.ID().Fqtn(), len(srcRows), len(dstRows))}, nil
	}

	srcSorted, err := sortByStableKey(srcRows)
	if err != nil {
		return 0, nil, err
	}
	dstSorted, err := sortByStableKey(dstRows)
	if err != nil {
		return 0, nil, err
	}

	mismatches := 0
	samples := make([]string, 0)
	for i := range srcSorted {
		srcRow := srcSorted[i]
		dstRow := dstSorted[i]
		srcKey, _ := rowStableKey(srcRow)
		dstKey, _ := rowStableKey(dstRow)
		if srcKey != dstKey {
			mismatches++
			if len(samples) < debugSampleRows {
				samples = append(samples, fmt.Sprintf("%s key mismatch src=%s dst=%s", table.ID().Fqtn(), srcKey, dstKey))
			}
			continue
		}

		srcByName := rowValuesByName(srcRow)
		dstByName := rowValuesByName(dstRow)
		for idx, colName := range srcRow.ColumnNames {
			srcVal := srcByName[colName]
			dstVal, ok := dstByName[colName]
			if !ok {
				mismatches++
				if len(samples) < debugSampleRows {
					samples = append(samples, fmt.Sprintf("%s key=%s missing column=%s in dst", table.ID().Fqtn(), srcKey, colName))
				}
				continue
			}
			var srcCol abstract.ColSchema
			var dstCol abstract.ColSchema
			if srcRow.TableSchema != nil && idx < len(srcRow.TableSchema.Columns()) {
				srcCol = srcRow.TableSchema.Columns()[idx]
			}
			if dstRow.TableSchema != nil && idx < len(dstRow.TableSchema.Columns()) {
				dstCol = dstRow.TableSchema.Columns()[idx]
			}
			equal, cmpErr := compareValuesStable(srcVal, srcCol, dstVal, dstCol, priorityComparators)
			if cmpErr != nil {
				return 0, nil, cmpErr
			}
			if !equal {
				mismatches++
				if len(samples) < debugSampleRows {
					samples = append(samples, fmt.Sprintf("%s key=%s column=%s src=%v dst=%v", table.ID().Fqtn(), srcKey, colName, srcVal, dstVal))
				}
			}
		}
	}

	return mismatches, samples, nil
}

func sortByStableKey(rows []abstract.ChangeItem) ([]abstract.ChangeItem, error) {
	out := append([]abstract.ChangeItem(nil), rows...)
	keys := make([]string, len(out))
	for i := range out {
		key, err := rowStableKey(out[i])
		if err != nil {
			return nil, err
		}
		keys[i] = key
	}
	sort.SliceStable(out, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return out, nil
}

func rowStableKey(row abstract.ChangeItem) (string, error) {
	if len(row.ColumnNames) != len(row.ColumnValues) {
		return "", errors.New("row has mismatched column names and values")
	}
	keyParts := make([]string, 0)
	schemaCols := []abstract.ColSchema(nil)
	if row.TableSchema != nil {
		schemaCols = row.TableSchema.Columns()
	}
	for i, name := range row.ColumnNames {
		isKey := false
		if i < len(schemaCols) {
			isKey = schemaCols[i].PrimaryKey || schemaCols[i].FakeKey
		}
		if isKey {
			keyParts = append(keyParts, fmt.Sprintf("%s=%v", name, row.ColumnValues[i]))
		}
	}
	if len(keyParts) == 0 {
		for i, name := range row.ColumnNames {
			keyParts = append(keyParts, fmt.Sprintf("%s=%v", name, row.ColumnValues[i]))
		}
	}
	return strings.Join(keyParts, ","), nil
}

func rowValuesByName(row abstract.ChangeItem) map[string]interface{} {
	res := make(map[string]interface{}, len(row.ColumnNames))
	for i, name := range row.ColumnNames {
		if i < len(row.ColumnValues) {
			res[name] = row.ColumnValues[i]
		}
	}
	return res
}

func compareValuesStable(
	lVal interface{},
	lSchema abstract.ColSchema,
	rVal interface{},
	rSchema abstract.ColSchema,
	priorityComparators []tasks.ChecksumComparator,
) (bool, error) {
	for _, comparator := range priorityComparators {
		comparable, result, err := comparator(lVal, lSchema, rVal, rSchema, false)
		if err != nil {
			return false, err
		}
		if comparable {
			return result, nil
		}
	}
	return fmt.Sprintf("%v", lVal) == fmt.Sprintf("%v", rVal), nil
}

func WaitStoragesSynced(t *testing.T, sourceModel, targetModel interface{}, retries uint64, compareParams *CompareStoragesParams) error {
	err := backoff.Retry(func() error {
		err := CompareStorages(t, sourceModel, targetModel, compareParams)
		if err != nil {
			logger.Log.Info("storage comparison failed", log.Error(err))
		}
		return err
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(2*time.Second), retries))
	return err
}

func CheckRowsCount(t *testing.T, serverModel interface{}, schema, tableName string, expectedRows uint64) {
	storage := GetSampleableStorageByModel(t, serverModel)
	tableDescr := abstract.TableDescription{Name: tableName, Schema: schema}
	rowsInSrc, err := storage.ExactTableRowsCount(tableDescr.ID())
	require.NoError(t, err)
	require.Equal(t, int(expectedRows), int(rowsInSrc))
}

func CheckRowsGreaterOrEqual(t *testing.T, serverModel interface{}, schema, tableName string, expectedMinumumRows uint64) {
	storage := GetSampleableStorageByModel(t, serverModel)
	tableDescr := abstract.TableDescription{Name: tableName, Schema: schema}
	rowsInSrc, err := storage.ExactTableRowsCount(tableDescr.ID())
	require.NoError(t, err)
	require.GreaterOrEqual(t, int(rowsInSrc), int(expectedMinumumRows))
}
