package documentshape

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
	target = chrecipe.MustTarget(chrecipe.WithInitFile(helpers.RepoPath("tests", "e2e-core", "mongo2ch", "snapshot", "dump.sql")), chrecipe.WithDatabase(databaseName))
)

func jsonAsStringComparator(lVal interface{}, _ abstract.ColSchema, rVal interface{}, _ abstract.ColSchema, _ bool) (bool, bool, error) {
	leftJSON, _ := json.Marshal(lVal)
	return true, string(leftJSON) == cast.ToString(rVal), nil
}

func TestDocumentShapeEvolution(t *testing.T) {
	defer func() {
		require.NoError(t, helpers.CheckConnections(
			helpers.LabeledPort{Label: "Mongo source", Port: source.Port},
			helpers.LabeledPort{Label: "CH HTTP target", Port: target.HTTPPort},
			helpers.LabeledPort{Label: "CH Native target", Port: target.NativePort},
		))
	}()

	testSource := *source
	testTarget := *target
	collectionName := fmt.Sprintf("shape_%d", time.Now().UnixNano())
	testSource.Collections = []mongocommon.MongoCollection{{
		DatabaseName:   databaseName,
		CollectionName: collectionName,
	}}

	require.NoError(t, mongocanon.InsertDocs(
		context.Background(),
		&testSource,
		databaseName,
		collectionName,
		bson.D{{Key: "_id", Value: "base_1"}, {Key: "event", Value: "base"}, {Key: "v", Value: 1}},
		bson.D{{Key: "_id", Value: "base_2"}, {Key: "event", Value: "base"}, {Key: "v", Value: 2}},
	))

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

	require.NoError(t, mongocanon.InsertDocs(
		context.Background(),
		&testSource,
		databaseName,
		collectionName,
		bson.D{{
			Key: "_id", Value: "evo_1",
		}, {
			Key: "event", Value: "evolved",
		}, {
			Key: "v", Value: 10,
		}, {
			Key: "profile", Value: bson.D{{Key: "region", Value: "us"}, {Key: "tier", Value: 3}},
		}, {
			Key: "flags", Value: bson.A{"f1", "f2"},
		}},
		bson.D{{
			Key: "_id", Value: "evo_2",
		}, {
			Key: "event", Value: "evolved",
		}, {
			Key: "v", Value: 11,
		}, {
			Key: "profile", Value: bson.D{{Key: "region", Value: "eu"}, {Key: "tier", Value: 2}},
		}, {
			Key: "meta", Value: bson.D{{Key: "source", Value: "new_schema"}},
		}},
	))

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		databaseName,
		collectionName,
		helpers.GetSampleableStorageByModel(t, &testSource),
		helpers.GetSampleableStorageByModel(t, &testTarget),
		120*time.Second,
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
