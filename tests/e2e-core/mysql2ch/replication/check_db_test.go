package snapshot

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/pkg/abstract"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	"github.com/transferia/transferia/pkg/providers/mysql"
	"github.com/transferia/transferia/tests/e2e-core/mysql2ch"
	"github.com/transferia/transferia/tests/e2e-core/pg2ch"
	"github.com/transferia/transferia/tests/helpers"
)

var (
	TransferType = abstract.TransferTypeSnapshotAndIncrement
	Source       = *helpers.RecipeMysqlSource()
	Target       = *chrecipe.MustTarget(chrecipe.WithInitFile("dump/ch/dump.sql"), chrecipe.WithDatabase("source"))
)

func init() {
	_ = os.Setenv("YC", "1") // to not go to vanga
	helpers.InitSrcDst(helpers.TransferID, &Source, &Target, TransferType)
}

func TestReplication(t *testing.T) {
	defer func() {
		require.NoError(t, helpers.CheckConnections(
			helpers.LabeledPort{Label: "Mysql source", Port: Source.Port},
			helpers.LabeledPort{Label: "CH target", Port: Target.NativePort},
		))
	}()

	transfer := helpers.MakeTransfer(helpers.TransferID, &Source, &Target, TransferType)
	worker := helpers.Activate(t, transfer)
	defer worker.Close(t)

	//------------------------------------------------------------------------------------
	// insert/update/delete several record

	connParams, err := mysql.NewConnectionParams(Source.ToStorageParams())
	require.NoError(t, err)
	client, err := mysql.Connect(connParams, nil)
	require.NoError(t, err)

	_, err = client.Exec("INSERT INTO mysql_replication (id, val1, val2, b1, b8, b11) VALUES (3, 3, 'c', NULL, NULL, NULL), (4, 4, 'd', b'0', b'00000000', b'00000000000'), (5, 5, 'e', b'1', b'11111111', b'11111111111')")
	require.NoError(t, err)

	_, err = client.Exec("UPDATE mysql_replication SET val2='ee' WHERE id=5;")
	require.NoError(t, err)

	_, err = client.Query("DELETE FROM mysql_replication WHERE id=3;")
	require.NoError(t, err)

	//------------------------------------------------------------------------------------

	require.NoError(t, helpers.WaitEqualRowsCount(t, Source.Database, "mysql_replication", helpers.GetSampleableStorageByModel(t, Source), helpers.GetSampleableStorageByModel(t, Target), 60*time.Second))
	require.NoError(t, helpers.CompareStorages(t, Source, Target, helpers.NewCompareStorageParams().WithEqualDataTypes(pg2ch.PG2CHDataTypesComparator).WithPriorityComparators(mysql2ch.MySQLBytesToStringOptionalComparator)))
}
