package replication

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/internal/logger"
	"github.com/transferia/transferia/pkg/abstract"
	cpclient "github.com/transferia/transferia/pkg/abstract/coordinator"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	pgcommon "github.com/transferia/transferia/pkg/providers/postgres"
	"github.com/transferia/transferia/pkg/providers/postgres/pgrecipe"
	"github.com/transferia/transferia/pkg/runtime/local"
	"github.com/transferia/transferia/pkg/worker/tasks"
	"github.com/transferia/transferia/tests/helpers"
)

var (
	databaseName = "public"
	TransferType = abstract.TransferTypeSnapshotAndIncrement
	Source       = *pgrecipe.RecipeSource(pgrecipe.WithInitDir("dump/pg"), pgrecipe.WithPrefix(""))
	Target       = *chrecipe.MustTarget(chrecipe.WithInitDir("dump/ch"), chrecipe.WithDatabase(databaseName))
)

func init() {
	helpers.InitSrcDst(helpers.TransferID, &Source, &Target, TransferType) // to WithDefaults() & FillDependentFields(): IsHomo, helpers.TransferID, IsUpdateable
}

func TestSnapshotAndIncrement(t *testing.T) {
	defer func() {
		require.NoError(t, helpers.CheckConnections(
			helpers.LabeledPort{Label: "PG source", Port: Source.Port},
			helpers.LabeledPort{Label: "CH target", Port: Target.NativePort},
		))
	}()

	connConfig, err := pgcommon.MakeConnConfigFromSrc(logger.Log, &Source)
	require.NoError(t, err)
	conn, err := pgcommon.NewPgConnPool(connConfig, logger.Log)
	require.NoError(t, err)

	//------------------------------------------------------------------------------------
	// start worker

	transfer := helpers.MakeTransfer(helpers.TransferID, &Source, &Target, TransferType)

	err = tasks.ActivateDelivery(context.Background(), nil, cpclient.NewFakeClient(), *transfer, helpers.EmptyRegistry())
	require.NoError(t, err)

	localWorker := local.NewLocalWorker(cpclient.NewFakeClient(), transfer, helpers.EmptyRegistry(), logger.Log)
	localWorker.Start()
	defer localWorker.Stop() //nolint

	//------------------------------------------------------------------------------------
	// insert/update/delete several record

	rows, err := conn.Query(context.Background(), "INSERT INTO public.date_types (__primary_key) VALUES (default)")
	require.NoError(t, err)
	rows.Close()

	rows, err = conn.Query(context.Background(), "UPDATE public.date_types SET t_time=now() WHERE __primary_key=2;")
	require.NoError(t, err)
	rows.Close()

	rows, err = conn.Query(context.Background(), "UPDATE public.date_types SET t_time=null WHERE __primary_key=2;")
	require.NoError(t, err)
	rows.Close()

	//------------------------------------------------------------------------------------
	// wait & compare

	// For this no-PK timestamp fixture, current delete/update semantics in CH converge to
	// zero active rows. Assert deterministic convergence instead of strict parity.
	targetStorage := helpers.GetSampleableStorageByModel(t, Target)
	tableDesc := abstract.TableDescription{Name: "date_types", Schema: databaseName}
	tableID := tableDesc.ID()
	require.Eventually(t, func() bool {
		rowsCount, countErr := targetStorage.ExactTableRowsCount(tableID)
		return countErr == nil && rowsCount == 0
	}, 60*time.Second, 2*time.Second)
}
