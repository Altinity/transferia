package highvolume

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/internal/logger"
	"github.com/transferia/transferia/pkg/abstract"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	pgcommon "github.com/transferia/transferia/pkg/providers/postgres"
	"github.com/transferia/transferia/pkg/providers/postgres/pgrecipe"
	"github.com/transferia/transferia/tests/e2e/pg2ch"
	"github.com/transferia/transferia/tests/helpers"
)

var (
	databaseName = "public"
	transferType = abstract.TransferTypeSnapshotAndIncrement
	source       = *pgrecipe.RecipeSource(pgrecipe.WithInitDir(helpers.RepoPath("tests", "e2e", "pg2ch", "replication", "dump", "pg")), pgrecipe.WithPrefix(""), pgrecipe.WithoutPgDump())
	target       = *chrecipe.MustTarget(chrecipe.WithInitDir(helpers.RepoPath("tests", "e2e", "pg2ch", "replication", "dump", "ch")), chrecipe.WithDatabase(databaseName))
)

func init() {
	_ = os.Setenv("YC", "1")
	helpers.InitSrcDst(helpers.TransferID, &source, &target, transferType)
}

func TestHighVolumeReplication(t *testing.T) {
	defer func() {
		require.NoError(t, helpers.CheckConnections(
			helpers.LabeledPort{Label: "PG source", Port: source.Port},
			helpers.LabeledPort{Label: "CH target", Port: target.NativePort},
		))
	}()

	connConfig, err := pgcommon.MakeConnConfigFromSrc(logger.Log, &source)
	require.NoError(t, err)
	conn, err := pgcommon.NewPgConnPool(connConfig, logger.Log)
	require.NoError(t, err)

	transfer := helpers.MakeTransfer(helpers.GenerateTransferID(t.Name()), &source, &target, transferType)
	worker := helpers.Activate(t, transfer)
	defer worker.Close(t)

	_, err = conn.Exec(context.Background(), `
INSERT INTO __test (id, val1, val2)
SELECT g, g * 10, 'bulk_' || g::text
FROM generate_series(1000, 3999) AS g`)
	require.NoError(t, err)

	_, err = conn.Exec(context.Background(), `UPDATE __test SET val1 = val1 + 7 WHERE id BETWEEN 1500 AND 2800`)
	require.NoError(t, err)

	_, err = conn.Exec(context.Background(), `DELETE FROM __test WHERE id BETWEEN 1000 AND 1199`)
	require.NoError(t, err)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		databaseName,
		"__test",
		helpers.GetSampleableStorageByModel(t, source),
		helpers.GetSampleableStorageByModel(t, target),
		180*time.Second,
	))

	require.NoError(t, helpers.CompareStorages(
		t,
		source,
		target,
		helpers.NewCompareStorageParams().WithEqualDataTypes(pg2ch.PG2CHDataTypesComparator),
	))
}
