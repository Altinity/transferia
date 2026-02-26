package snapshot

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/pkg/abstract"
	dpmodel "github.com/transferia/transferia/pkg/abstract/model"
	"github.com/transferia/transferia/pkg/providers/clickhouse/model"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	mongocommon "github.com/transferia/transferia/pkg/providers/mongo"
	"github.com/transferia/transferia/pkg/transformer"
	"github.com/transferia/transferia/pkg/transformer/registry/clickhouse"
	"github.com/transferia/transferia/pkg/transformer/registry/filter"
	"github.com/transferia/transferia/tests/canon/mongo"
	"github.com/transferia/transferia/tests/helpers"
	"go.mongodb.org/mongo-driver/bson"
)

const flattenDatabaseName string = "db"

var (
	source = mongocommon.RecipeSource()
	target = chrecipe.MustTarget(chrecipe.WithInitFile(helpers.RepoPath("tests", "e2e-core", "mongo2ch", "snapshot_flatten", "dump.sql")), chrecipe.WithDatabase(flattenDatabaseName))
)

func TestResumeFromCoordinator(t *testing.T) {
	src := *source
	dst := *target
	dst.ChClusterName = ""

	collectionName := fmt.Sprintf("flatten_resume_%d", time.Now().UnixNano())
	src.Collections = []mongocommon.MongoCollection{
		{DatabaseName: flattenDatabaseName, CollectionName: collectionName},
	}

	seedDoc := parseJSONDoc(t, `{
	  "_id": "seed",
	  "floors": [
	    {"currency":"EUR","value":0.2,"countryIds":["IT"]}
	  ]
	}`)
	require.NoError(t, mongo.InsertDocs(context.Background(), &src, flattenDatabaseName, collectionName, seedDoc))

	transferID := helpers.GenerateTransferID(t.Name())
	transfer := newFlattenTransfer(transferID, &src, &dst, collectionName)
	cp := helpers.NewCoordinatorForTransfer(t, transferID)

	worker, err := helpers.ActivateWithCP(transfer, cp, true)
	require.NoError(t, err)
	defer worker.Close(t)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		flattenDatabaseName,
		collectionName,
		helpers.GetSampleableStorageByModel(t, &src),
		helpers.GetSampleableStorageByModel(t, &dst),
		120*time.Second,
	))

	worker.Close(t)

	incDoc := parseJSONDoc(t, `{
	  "_id": "inc",
	  "floors": [
	    {"currency":"USD","value":0.7,"countryIds":["US","CA"]}
	  ]
	}`)
	require.NoError(t, mongo.InsertDocs(context.Background(), &src, flattenDatabaseName, collectionName, incDoc))

	worker.Restart(t, transfer)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		flattenDatabaseName,
		collectionName,
		helpers.GetSampleableStorageByModel(t, &src),
		helpers.GetSampleableStorageByModel(t, &dst),
		120*time.Second,
	))

	rows := helpers.LoadTable(
		t,
		helpers.GetSampleableStorageByModel(t, &dst),
		abstract.TableDescription{Schema: flattenDatabaseName, Name: collectionName},
	)
	require.Len(t, rows, 2)

	ids := map[string]bool{}
	for _, row := range rows {
		vals := row.AsMap()
		id := fmt.Sprint(vals["_id"])
		ids[id] = true
		require.Contains(t, vals, "currency_from_floors")
		require.NotNil(t, vals["currency_from_floors"])
	}
	require.True(t, ids["seed"])
	require.True(t, ids["inc"])
}

func parseJSONDoc(t *testing.T, doc string) bson.D {
	t.Helper()
	var out bson.D
	require.NoError(t, bson.UnmarshalExtJSON([]byte(doc), false, &out))
	return out
}

func newFlattenTransfer(transferID string, src *mongocommon.MongoSource, dst *model.ChDestination, collection string) *dpmodel.Transfer {
	transfer := helpers.MakeTransfer(transferID, src, dst, abstract.TransferTypeSnapshotAndIncrement)
	transfer.TypeSystemVersion = 7
	transfer.Transformation = &dpmodel.Transformation{Transformers: &transformer.Transformers{
		DebugMode: false,
		Transformers: []transformer.Transformer{{
			clickhouse.Type: clickhouse.Config{
				Tables: filter.Tables{
					IncludeTables: []string{fmt.Sprintf("%s.%s", flattenDatabaseName, collection)},
				},
				Query: `
SELECT _id,
	JSONExtractArrayRaw(document,'floors') as floors_as_string_array,
	arrayMap(x -> JSONExtractFloat(x, 'value'), JSONExtractArrayRaw(document,'floors')) as value_from_floors,
	arrayMap(x -> JSONExtractString(x, 'currency'), JSONExtractArrayRaw(document,'floors')) as currency_from_floors,
	JSONExtractRaw(assumeNotNull(document),'floors') AS floors_as_string
FROM table
SETTINGS
	function_json_value_return_type_allow_nullable = true,
	function_json_value_return_type_allow_complex = true
`,
			},
		}},
		ErrorsOutput: nil,
	}}
	return transfer
}
