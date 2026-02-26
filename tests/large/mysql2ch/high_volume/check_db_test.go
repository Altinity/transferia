package highvolume

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/pkg/abstract"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	"github.com/transferia/transferia/pkg/providers/mysql"
	mysqlcomparators "github.com/transferia/transferia/tests/e2e-core/mysql2ch"
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

func insertRange(t *testing.T, client *sql.DB, startID, endID int) {
	t.Helper()
	const batchSize = 250

	for from := startID; from <= endID; from += batchSize {
		to := from + batchSize - 1
		if to > endID {
			to = endID
		}

		builder := strings.Builder{}
		builder.WriteString("INSERT INTO mysql_replication (id, val1, val2, b1, b8, b11) VALUES ")
		for id := from; id <= to; id++ {
			if id > from {
				builder.WriteString(",")
			}
			bit := id % 2
			fmt.Fprintf(&builder, "(%d, %d, 'bulk_%d', b'%d', b'00000001', b'00000000001')", id, id, id, bit)
		}

		_, err := client.Exec(builder.String())
		require.NoError(t, err)
	}
}

func TestHighVolumeReplication(t *testing.T) {
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

	insertRange(t, client, 1000, 3999)

	_, err = client.Exec("UPDATE mysql_replication SET val2=CONCAT(val2, '_u') WHERE id BETWEEN 1400 AND 2600")
	require.NoError(t, err)

	_, err = client.Query("DELETE FROM mysql_replication WHERE id BETWEEN 1000 AND 1150")
	require.NoError(t, err)

	require.NoError(t, helpers.WaitEqualRowsCount(
		t,
		source.Database,
		"mysql_replication",
		helpers.GetSampleableStorageByModel(t, source),
		helpers.GetSampleableStorageByModel(t, target),
		180*time.Second,
	))

	require.NoError(t, helpers.CompareStorages(
		t,
		source,
		target,
		helpers.NewCompareStorageParams().
			WithEqualDataTypes(pg2ch.PG2CHDataTypesComparator).
			WithPriorityComparators(mysqlcomparators.MySQLBytesToStringOptionalComparator),
	))
}
