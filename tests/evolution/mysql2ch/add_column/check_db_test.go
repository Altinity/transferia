package addcolumn

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
	transferType = abstract.TransferTypeSnapshotAndIncrement
	source       = *helpers.RecipeMysqlSource()
	target       = *chrecipe.MustTarget(chrecipe.WithInitDir(helpers.RepoPath("tests", "e2e-core", "mysql2ch", "replication", "dump", "ch")), chrecipe.WithDatabase("source"))
)

func init() {
	_ = os.Setenv("YC", "1")
	helpers.InitSrcDst(helpers.TransferID, &source, &target, transferType)
}

func TestAddColumnDuringReplication(t *testing.T) {
	defer func() {
		require.NoError(t, helpers.CheckConnections(
			helpers.LabeledPort{Label: "Mysql source", Port: source.Port},
			helpers.LabeledPort{Label: "CH target", Port: target.NativePort},
		))
	}()

	transfer := helpers.MakeTransfer(helpers.GenerateTransferID(t.Name()), &source, &target, transferType)
	worker := helpers.Activate(t, transfer)
	defer worker.Close(t)

	connParams, err := mysql.NewConnectionParams(source.ToStorageParams())
	require.NoError(t, err)

	client, err := mysql.Connect(connParams, nil)
	require.NoError(t, err)

	_, err = client.Exec("ALTER TABLE mysql_replication ADD COLUMN evolution_metric INT NULL")
	require.NoError(t, err)

	_, err = client.Exec("INSERT INTO mysql_replication (id, val1, val2, b1, b8, b11, evolution_metric) VALUES (101, 101, 'evo_a', b'1', b'00000001', b'00000000001', 501), (102, 102, 'evo_b', b'0', b'00000000', b'00000000000', 502)")
	require.NoError(t, err)

	_, err = client.Exec("UPDATE mysql_replication SET evolution_metric = 700 WHERE id = 1")
	require.NoError(t, err)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		source.Database,
		"mysql_replication",
		helpers.GetSampleableStorageByModel(t, source),
		helpers.GetSampleableStorageByModel(t, target),
		90*time.Second,
	))

	require.NoError(t, helpers.CompareStorages(
		t,
		source,
		target,
		helpers.NewCompareStorageParams().
			WithEqualDataTypes(pg2ch.PG2CHDataTypesComparator).
			WithPriorityComparators(mysql2ch.MySQLBytesToStringOptionalComparator),
	))
}
