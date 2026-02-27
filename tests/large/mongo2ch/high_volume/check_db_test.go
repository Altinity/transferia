package highvolume

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/cast"
	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/pkg/abstract"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	mongocommon "github.com/transferia/transferia/pkg/providers/mongo"
	mongocanon "github.com/transferia/transferia/tests/canon/mongo"
	"github.com/transferia/transferia/tests/helpers"
	"go.mongodb.org/mongo-driver/bson"
)

const databaseName = "db"

var (
	source = mongocommon.RecipeSource()
	target = chrecipe.MustTarget(chrecipe.WithInitFile(helpers.RepoPath("tests", "e2e", "mongo2ch", "snapshot", "dump.sql")), chrecipe.WithDatabase(databaseName))
)

func jsonAsStringComparator(lVal interface{}, _ abstract.ColSchema, rVal interface{}, _ abstract.ColSchema, _ bool) (bool, bool, error) {
	leftJSON, _ := json.Marshal(lVal)
	return true, string(leftJSON) == cast.ToString(rVal), nil
}

func buildDocs(prefix string, start, end int) []any {
	result := make([]any, 0, end-start+1)
	for i := start; i <= end; i++ {
		result = append(result, bson.D{
			{Key: "_id", Value: fmt.Sprintf("%s_%d", prefix, i)},
			{Key: "seq", Value: i},
			{Key: "payload", Value: bson.D{{Key: "bucket", Value: i % 17}, {Key: "nested", Value: bson.D{{Key: "v", Value: i}, {Key: "arr", Value: bson.A{i, i + 1, i + 2}}}}}},
		})
	}
	return result
}

func TestHighVolumeReplication(t *testing.T) {
	defer func() {
		require.NoError(t, helpers.CheckConnections(
			helpers.LabeledPort{Label: "Mongo source", Port: source.Port},
			helpers.LabeledPort{Label: "CH HTTP target", Port: target.HTTPPort},
			helpers.LabeledPort{Label: "CH Native target", Port: target.NativePort},
		))
	}()

	testSource := *source
	testTarget := *target
	collectionName := fmt.Sprintf("bulk_%d", time.Now().UnixNano())
	testSource.Collections = []mongocommon.MongoCollection{{
		DatabaseName:   databaseName,
		CollectionName: collectionName,
	}}

	require.NoError(t, mongocanon.InsertDocs(context.Background(), &testSource, databaseName, collectionName, buildDocs("seed", 1, 500)...))

	transfer := helpers.MakeTransfer(helpers.GenerateTransferID(t.Name()), &testSource, &testTarget, abstract.TransferTypeSnapshotAndIncrement)
	transfer.TypeSystemVersion = 7

	worker := helpers.Activate(t, transfer)
	defer worker.Close(t)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		databaseName,
		collectionName,
		helpers.GetSampleableStorageByModel(t, &testSource),
		helpers.GetSampleableStorageByModel(t, &testTarget),
		120*time.Second,
	))

	require.NoError(t, mongocanon.InsertDocs(context.Background(), &testSource, databaseName, collectionName, buildDocs("bulk", 501, 2500)...))

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		databaseName,
		collectionName,
		helpers.GetSampleableStorageByModel(t, &testSource),
		helpers.GetSampleableStorageByModel(t, &testTarget),
		180*time.Second,
	))

	require.NoError(t, helpers.CompareStorages(
		t,
		&testSource,
		&testTarget,
		helpers.NewCompareStorageParams().
			WithEqualDataTypes(func(_, _ string) bool { return true }).
			WithPriorityComparators(jsonAsStringComparator),
	))
}
