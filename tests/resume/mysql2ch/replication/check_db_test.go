package replication

import (
	"fmt"
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

func TestResumeFromCoordinator(t *testing.T) {
	defer func() {
		require.NoError(t, helpers.CheckConnections(
			helpers.LabeledPort{Label: "Mysql source", Port: source.Port},
			helpers.LabeledPort{Label: "CH target", Port: target.NativePort},
		))
	}()

	connParams, err := mysql.NewConnectionParams(source.ToStorageParams())
	require.NoError(t, err)
	client, err := mysql.Connect(connParams, nil)
	require.NoError(t, err)

	_, err = client.Exec("DROP TABLE IF EXISTS mysql_replication")
	require.NoError(t, err)
	_, err = client.Exec(`
		CREATE TABLE mysql_replication (
			id INT AUTO_INCREMENT PRIMARY KEY,
			val1 INT,
			val2 VARCHAR(20),
			b1 BIT(1),
			b8 BIT(8),
			b11 BIT(11)
		) engine = innodb default charset = utf8;
	`)
	require.NoError(t, err)
	_, err = client.Exec("INSERT INTO mysql_replication (id, val1, val2, b1, b8, b11) VALUES (1, 1, 'a', b'0', b'00000000', b'00000000000'), (2, 2, 'b', b'1', b'10000000', b'10000000000')")
	require.NoError(t, err)

	transferID := helpers.GenerateTransferID(t.Name())
	transfer := helpers.MakeTransfer(transferID, &source, &target, transferType)
	cp := helpers.NewCoordinatorForTransfer(t, transferID)

	worker, err := helpers.ActivateWithCP(transfer, cp, true)
	require.NoError(t, err)
	defer worker.Close(t)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		source.Database,
		"mysql_replication",
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
			WithPriorityComparators(mysql2ch.MySQLBytesToStringOptionalComparator).
			WithStableFallback(true),
	))

	worker.Close(t)

	resumeID := int(time.Now().UnixNano()%1_000_000) + 1000
	_, err = client.Exec(fmt.Sprintf(
		"INSERT INTO mysql_replication (id, val1, val2, b1, b8, b11) VALUES (%d, %d, 'resume', NULL, NULL, NULL)",
		resumeID,
		resumeID,
	))
	require.NoError(t, err)

	worker.Restart(t, transfer)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		source.Database,
		"mysql_replication",
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
			WithPriorityComparators(mysql2ch.MySQLBytesToStringOptionalComparator).
			WithStableFallback(true),
	))
}
