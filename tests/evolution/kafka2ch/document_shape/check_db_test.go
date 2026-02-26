package documentshape

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/internal/logger"
	"github.com/transferia/transferia/library/go/core/metrics/solomon"
	"github.com/transferia/transferia/pkg/abstract"
	"github.com/transferia/transferia/pkg/abstract/model"
	"github.com/transferia/transferia/pkg/parsers"
	jsonparser "github.com/transferia/transferia/pkg/parsers/registry/json"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	kafkasink "github.com/transferia/transferia/pkg/providers/kafka"
	"github.com/transferia/transferia/tests/helpers"
	ytschema "go.ytsaurus.tech/yt/go/schema"
)

func mustKafkaJSONParserConfig(t *testing.T) map[string]interface{} {
	t.Helper()
	parserCfg := &jsonparser.ParserConfigJSONCommon{
		Fields: []abstract.ColSchema{
			{ColumnName: "id", DataType: ytschema.TypeInt32.String(), PrimaryKey: true},
			{ColumnName: "msg", DataType: ytschema.TypeString.String()},
		},
		AddRest:       true,
		AddDedupeKeys: true,
	}
	cfg, err := parsers.ParserConfigStructToMap(parserCfg)
	require.NoError(t, err)
	return cfg
}

func mustKafkaSink(t *testing.T, src *kafkasink.KafkaSource) abstract.Sinker {
	t.Helper()
	sink, err := kafkasink.NewReplicationSink(
		&kafkasink.KafkaDestination{
			Connection: src.Connection,
			Auth:       src.Auth,
			Topic:      src.Topic,
			FormatSettings: model.SerializationFormat{
				Name: model.SerializationFormatMirror,
				BatchingSettings: &model.Batching{
					Enabled: false,
				},
			},
			ParralelWriterCount: 4,
		},
		solomon.NewRegistry(nil).WithTags(map[string]string{"ts": time.Now().String()}),
		logger.Log,
	)
	require.NoError(t, err)
	return sink
}

func TestDocumentShapeEvolution(t *testing.T) {
	source := *kafkasink.MustSourceRecipe()
	source.Topic = fmt.Sprintf("kafka_shape_%d", time.Now().UnixNano())
	source.ParserConfig = mustKafkaJSONParserConfig(t)

	target := *chrecipe.MustTarget(chrecipe.WithInitDir("dump/ch"), chrecipe.WithDatabase("public"))

	transfer := helpers.MakeTransfer(helpers.GenerateTransferID(t.Name()), &source, &target, abstract.TransferTypeIncrementOnly)
	worker := helpers.Activate(t, transfer)
	defer worker.Close(t)

	sink := mustKafkaSink(t, &source)

	push := func(offset int64, payload map[string]any) {
		t.Helper()
		v, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, sink.Push([]abstract.ChangeItem{
			abstract.MakeRawMessage([]byte("_"), source.Topic, time.Time{}, source.Topic, 0, int64(offset), v),
		}))
	}

	push(0, map[string]any{"id": 1, "msg": "base"})
	push(1, map[string]any{"id": 2, "msg": "base2"})

	require.NoError(t, helpers.WaitDestinationEqualRowsCount(
		"public",
		source.Topic,
		helpers.GetSampleableStorageByModel(t, target),
		60*time.Second,
		2,
	))

	push(2, map[string]any{"id": 3, "msg": "evolved", "extra": map[string]any{"region": "us", "tier": 3}})
	push(3, map[string]any{"id": 4, "msg": "evolved2", "flags": []string{"f1", "f2"}})

	require.NoError(t, helpers.WaitDestinationEqualRowsCount(
		"public",
		source.Topic,
		helpers.GetSampleableStorageByModel(t, target),
		60*time.Second,
		4,
	))
}
