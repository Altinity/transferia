package replication

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/internal/logger"
	"github.com/transferia/transferia/pkg/abstract"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	pgcommon "github.com/transferia/transferia/pkg/providers/postgres"
	"github.com/transferia/transferia/pkg/providers/postgres/pgrecipe"
	"github.com/transferia/transferia/tests/e2e-core/pg2ch"
	"github.com/transferia/transferia/tests/helpers"
)

var (
	databaseName = "public"
	transferType = abstract.TransferTypeSnapshotAndIncrement
	source       = *pgrecipe.RecipeSource(
		pgrecipe.WithInitDir(helpers.RepoPath("tests", "e2e-core", "pg2ch", "replication", "dump", "pg")),
		pgrecipe.WithPrefix(""),
		pgrecipe.WithoutPgDump(),
	)
	target = *chrecipe.MustTarget(
		chrecipe.WithInitDir(helpers.RepoPath("tests", "e2e-core", "pg2ch", "replication", "dump", "ch")),
		chrecipe.WithDatabase(databaseName),
	)
)

func init() {
	_ = os.Setenv("YC", "1")
	helpers.InitSrcDst(helpers.TransferID, &source, &target, transferType)
}

func TestResumeFromCoordinator(t *testing.T) {
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

	transferID := helpers.GenerateTransferID(t.Name())
	transfer := helpers.MakeTransfer(transferID, &source, &target, transferType)
	cp := helpers.NewCoordinatorForTransfer(t, transferID)

	worker, err := helpers.ActivateWithCP(transfer, cp, true)
	require.NoError(t, err)
	defer worker.Close(t)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		databaseName,
		"__test",
		helpers.GetSampleableStorageByModel(t, source),
		helpers.GetSampleableStorageByModel(t, target),
		60*time.Second,
	))

	worker.Close(t)

	resumeID := int(time.Now().UnixNano()%1_000_000) + 1000
	_, err = conn.Exec(
		context.Background(),
		fmt.Sprintf("INSERT INTO __test (id, val1, val2) VALUES (%d, %d, 'resume')", resumeID, resumeID),
	)
	require.NoError(t, err)

	worker.Restart(t, transfer)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		databaseName,
		"__test",
		helpers.GetSampleableStorageByModel(t, source),
		helpers.GetSampleableStorageByModel(t, target),
		60*time.Second,
	))
	require.NoError(t, helpers.CompareStorages(
		t,
		source,
		target,
		helpers.NewCompareStorageParams().
			WithEqualDataTypes(pg2ch.PG2CHDataTypesComparator).
			WithStableFallback(true),
	))
}
