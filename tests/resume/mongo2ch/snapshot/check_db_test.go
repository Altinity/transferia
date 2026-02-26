package snapshot

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
	"github.com/transferia/transferia/tests/canon/mongo"
	"github.com/transferia/transferia/tests/helpers"
	"go.mongodb.org/mongo-driver/bson"
)

const databaseName string = "db"

var (
	source = mongocommon.RecipeSource()
	target = chrecipe.MustTarget(chrecipe.WithInitFile(helpers.RepoPath("tests", "e2e-core", "mongo2ch", "snapshot", "dump.sql")), chrecipe.WithDatabase(databaseName))
)

func TestResumeFromCoordinator(t *testing.T) {
	src := *source
	dst := *target

	collectionName := fmt.Sprintf("resume_%d", time.Now().UnixNano())
	src.Collections = []mongocommon.MongoCollection{
		{DatabaseName: databaseName, CollectionName: collectionName},
	}

	require.NoError(t, mongo.InsertDocs(
		context.Background(),
		&src,
		databaseName,
		collectionName,
		bson.D{{Key: "_id", Value: "baseline"}, {Key: "v", Value: 1}},
	))

	transferID := helpers.GenerateTransferID(t.Name())
	transfer := helpers.MakeTransfer(transferID, &src, &dst, abstract.TransferTypeSnapshotAndIncrement)
	transfer.TypeSystemVersion = 7

	cp := helpers.NewCoordinatorForTransfer(t, transferID)
	worker, err := helpers.ActivateWithCP(transfer, cp, true)
	require.NoError(t, err)
	defer worker.Close(t)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		databaseName,
		collectionName,
		helpers.GetSampleableStorageByModel(t, &src),
		helpers.GetSampleableStorageByModel(t, &dst),
		120*time.Second,
	))

	worker.Close(t)

	require.NoError(t, mongo.InsertDocs(
		context.Background(),
		&src,
		databaseName,
		collectionName,
		bson.D{{Key: "_id", Value: "increment"}, {Key: "v", Value: 2}},
	))

	worker.Restart(t, transfer)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		databaseName,
		collectionName,
		helpers.GetSampleableStorageByModel(t, &src),
		helpers.GetSampleableStorageByModel(t, &dst),
		120*time.Second,
	))

	require.NoError(t, helpers.CompareStorages(
		t,
		&src,
		&dst,
		helpers.NewCompareStorageParams().
			WithEqualDataTypes(func(_, _ string) bool {
				return true
			}).
			WithPriorityComparators(func(lVal interface{}, lSchema abstract.ColSchema, rVal interface{}, rSchema abstract.ColSchema, intoArray bool) (comparable bool, result bool, err error) {
				ld, _ := json.Marshal(lVal)
				return true, string(ld) == cast.ToString(rVal), nil
			}).
			WithStableFallback(true),
	))
}
