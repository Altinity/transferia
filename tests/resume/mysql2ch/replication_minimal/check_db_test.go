package snapshot

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/internal/logger"
	"github.com/transferia/transferia/pkg/abstract"
	cpclient "github.com/transferia/transferia/pkg/abstract/coordinator"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	"github.com/transferia/transferia/pkg/providers/mysql"
	"github.com/transferia/transferia/pkg/runtime/local"
	"github.com/transferia/transferia/tests/helpers"
)

var (
	databaseName = "source"
	TransferType = abstract.TransferTypeSnapshotAndIncrement
	Source       = *helpers.RecipeMysqlSource()
	Target       = *chrecipe.MustTarget(chrecipe.WithInitFile("dump/ch/dump.sql"), chrecipe.WithDatabase(databaseName))
)

func init() {
	_ = os.Setenv("YC", "1")                                               // to not go to vanga
	helpers.InitSrcDst(helpers.TransferID, &Source, &Target, TransferType) // to WithDefaults() & FillDependentFields(): IsHomo, helpers.TransferID, IsUpdateable
}

func TestReplication(t *testing.T) {
	defer func() {
		require.NoError(t, helpers.CheckConnections(
			helpers.LabeledPort{Label: "Mysql source", Port: Source.Port},
			helpers.LabeledPort{Label: "CH target", Port: Target.NativePort},
		))
	}()

	connParams, err := mysql.NewConnectionParams(Source.ToStorageParams())
	require.NoError(t, err)

	client, err := mysql.Connect(connParams, nil)
	require.NoError(t, err)

	//------------------------------------------------------------------------------------
	// start worker
	transfer := helpers.MakeTransfer(helpers.TransferID, &Source, &Target, TransferType)

	fakeClient := cpclient.NewStatefulFakeClient()
	err = mysql.SyncBinlogPosition(&Source, transfer.ID, fakeClient)
	require.NoError(t, err)

	localWorker := local.NewLocalWorker(fakeClient, transfer, helpers.EmptyRegistry(), logger.Log)
	localWorker.Start()
	defer localWorker.Stop() //nolint

	//------------------------------------------------------------------------------------
	// insert/update/delete several record

	rows, err := client.Query("INSERT INTO __test (id, val1, val2) VALUES (3, 3, 'c'), (4, 4, 'd'), (5, 5, 'e')")
	require.NoError(t, err)
	_ = rows.Close()

	rows, err = client.Query("UPDATE __test SET val2='ee' WHERE id=5;")
	require.NoError(t, err)
	_ = rows.Close()

	rows, err = client.Query("DELETE FROM __test WHERE id=3;")
	require.NoError(t, err)
	_ = rows.Close()

	//------------------------------------------------------------------------------------
	// wait & compare

	require.NoError(t, helpers.WaitEqualRowsCount(t, databaseName, "__test", helpers.GetSampleableStorageByModel(t, Source), helpers.GetSampleableStorageByModel(t, Target), 60*time.Second))
	require.NoError(t, helpers.CompareStorages(t, Source, Target, helpers.NewCompareStorageParams().WithEqualDataTypes(compareDataTypes)))
}

func compareDataTypes(l, r string) bool {
	if l == "utf8" && r == "string" {
		return true
	}
	return l == r
}
