package mongo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/pkg/abstract"
	"github.com/transferia/transferia/pkg/abstract/model"
	mongocommon "github.com/transferia/transferia/pkg/providers/mongo"
	"github.com/transferia/transferia/tests/canon/validator"
	"github.com/transferia/transferia/tests/helpers"
	"github.com/transferia/transferia/tests/tcrecipes"
)

func TestCanonSource(t *testing.T) {
	t.Setenv("YC", "1") // to not go to vanga
	if !tcrecipes.Enabled() {
		helpers.SkipIfMissingEnv(t, "MONGO_LOCAL_PORT", "MONGO_LOCAL_USER", "MONGO_LOCAL_PASSWORD")
	}
	databaseName := "canondb"
	t.Run("vanilla hetero case", func(t *testing.T) {
		snapshotPlusIncrementScenario(t, databaseName, "hetero_repack", false, false)
	})
	t.Run("strange hetero case", func(t *testing.T) {
		snapshotPlusIncrementScenario(t, databaseName, "hetero_no_repack", false, true)
	})
}

func snapshotPlusIncrementScenario(t *testing.T, databaseName, collectionName string, isHomo, preventJSONRepack bool) {
	Source := mongocommon.RecipeSource(mongocommon.WithCollections(
		mongocommon.MongoCollection{DatabaseName: databaseName, CollectionName: collectionName},
	))
	Source.IsHomo = isHomo
	Source.PreventJSONRepack = preventJSONRepack
	defer func() {
		require.NoError(t, helpers.CheckConnections(
			helpers.LabeledPort{Label: "Mongo source", Port: Source.Port},
		))
	}()

	ctx := context.Background()
	require.NoError(t, DropCollection(ctx, Source, databaseName, collectionName))

	require.NoError(t, InsertDocs(ctx, Source, databaseName, collectionName, SnapshotDocuments...))
	if !preventJSONRepack {
		require.NoError(t, InsertDocs(ctx, Source, databaseName, collectionName, ExtraSnapshotDocuments...))
	}

	transfer := helpers.MakeTransfer(
		helpers.TransferID,
		Source,
		&model.MockDestination{
			SinkerFactory: validator.New(
				model.IsStrictSource(Source),
				validator.InitDone(t),
				validator.ValuesTypeChecker,
				validator.Referencer(t),
				validator.TypesystemChecker(mongocommon.ProviderType, func(colSchema abstract.ColSchema) string {
					return strings.TrimPrefix(colSchema.OriginalType, "mongo:")
				}),
			),
			Cleanup: model.Drop,
		},
		abstract.TransferTypeSnapshotAndIncrement,
	)
	worker := helpers.Activate(t, transfer)
	defer worker.Close(t)

	time.Sleep(1 * time.Second)

	require.NoError(t, InsertDocs(ctx, Source, databaseName, collectionName, IncrementDocuments...))

	require.NoError(t, UpdateDocs(ctx, Source, databaseName, collectionName, IncrementUpdates...))

	time.Sleep(4 * mongocommon.DefaultBatchFlushInterval)
}
