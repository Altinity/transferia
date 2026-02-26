package helpers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/pkg/abstract"
	ytschema "go.ytsaurus.tech/yt/go/schema"
)

func TestCompareLoadedRowsStableOrderOnlyDifference(t *testing.T) {
	table := abstract.TableDescription{Schema: "public", Name: "stable_order"}
	schema := abstract.NewTableSchema([]abstract.ColSchema{
		{TableSchema: "public", TableName: "stable_order", ColumnName: "id", DataType: ytschema.TypeInt64.String(), PrimaryKey: true},
		{TableSchema: "public", TableName: "stable_order", ColumnName: "val", DataType: ytschema.TypeString.String()},
	})

	srcRows := []abstract.ChangeItem{
		makeStableRow(schema, 2, "b"),
		makeStableRow(schema, 1, "a"),
	}
	dstRows := []abstract.ChangeItem{
		makeStableRow(schema, 1, "a"),
		makeStableRow(schema, 2, "b"),
	}

	mismatches, samples, err := compareLoadedRowsStable(table, srcRows, dstRows, nil, 10)
	require.NoError(t, err)
	require.Equal(t, 0, mismatches)
	require.Empty(t, samples)
}

func TestCompareLoadedRowsStableValueMismatch(t *testing.T) {
	table := abstract.TableDescription{Schema: "public", Name: "stable_mismatch"}
	schema := abstract.NewTableSchema([]abstract.ColSchema{
		{TableSchema: "public", TableName: "stable_mismatch", ColumnName: "id", DataType: ytschema.TypeInt64.String(), PrimaryKey: true},
		{TableSchema: "public", TableName: "stable_mismatch", ColumnName: "val", DataType: ytschema.TypeString.String()},
	})

	srcRows := []abstract.ChangeItem{
		makeStableRow(schema, 1, "a"),
		makeStableRow(schema, 2, "b"),
	}
	dstRows := []abstract.ChangeItem{
		makeStableRow(schema, 1, "a"),
		makeStableRow(schema, 2, "DIFF"),
	}

	mismatches, samples, err := compareLoadedRowsStable(table, srcRows, dstRows, nil, 10)
	require.NoError(t, err)
	require.Equal(t, 1, mismatches)
	require.Len(t, samples, 1)
	require.Contains(t, samples[0], "stable_mismatch")
	require.Contains(t, samples[0], "key=id=2")
	require.Contains(t, samples[0], "column=val")
}

func TestApplyStableFallbackDisabledKeepsChecksumError(t *testing.T) {
	checksumErr := errors.New("checksum failed")
	params := NewCompareStorageParams().WithStableFallback(false)
	fallbackCalled := false

	err := applyStableFallback(checksumErr, params, func() error {
		fallbackCalled = true
		return nil
	})
	require.ErrorIs(t, err, checksumErr)
	require.False(t, fallbackCalled)
}

func makeStableRow(schema *abstract.TableSchema, id int64, val string) abstract.ChangeItem {
	return abstract.ChangeItem{
		Kind:         abstract.InsertKind,
		ColumnNames:  []string{"id", "val"},
		ColumnValues: []interface{}{id, val},
		TableSchema:  schema,
	}
}
