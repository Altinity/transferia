package highvolume

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

func parserCfg(t *testing.T) map[string]interface{} {
	t.Helper()
	parserCfg := &jsonparser.ParserConfigJSONCommon{
		Fields: []abstract.ColSchema{
			{ColumnName: "id", DataType: ytschema.TypeInt32.String(), PrimaryKey: true},
			{ColumnName: "msg", DataType: ytschema.TypeString.String()},
		},
		AddRest:       false,
		AddDedupeKeys: true,
	}
	cfg, err := parsers.ParserConfigStructToMap(parserCfg)
	require.NoError(t, err)
	return cfg
}

func newKafkaSink(t *testing.T, src *kafkasink.KafkaSource) abstract.Sinker {
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
			ParralelWriterCount: 8,
		},
		solomon.NewRegistry(nil).WithTags(map[string]string{"ts": time.Now().String()}),
		logger.Log,
	)
	require.NoError(t, err)
	return sink
}

func TestHighVolumeReplication(t *testing.T) {
	source := *kafkasink.MustSourceRecipe()
	source.Topic = fmt.Sprintf("kafka_bulk_%d", time.Now().UnixNano())
	source.ParserConfig = parserCfg(t)

	target := *chrecipe.MustTarget(chrecipe.WithInitDir("dump/ch"), chrecipe.WithDatabase("public"))

	transfer := helpers.MakeTransfer(helpers.GenerateTransferID(t.Name()), &source, &target, abstract.TransferTypeIncrementOnly)
	worker := helpers.Activate(t, transfer)
	defer worker.Close(t)

	sink := newKafkaSink(t, &source)

	const total = 1500
	batch := make([]abstract.ChangeItem, 0, 200)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		require.NoError(t, sink.Push(batch))
		batch = batch[:0]
	}

	for i := 0; i < total; i++ {
		v, err := json.Marshal(map[string]any{
			"id":  i + 1,
			"msg": fmt.Sprintf("bulk_%d", i+1),
		})
		require.NoError(t, err)
		batch = append(batch, abstract.MakeRawMessage([]byte("_"), source.Topic, time.Time{}, source.Topic, 0, int64(i), v))
		if len(batch) == cap(batch) {
			flush()
		}
	}
	flush()

	require.NoError(t, helpers.WaitDestinationEqualRowsCount(
		"public",
		source.Topic,
		helpers.GetSampleableStorageByModel(t, target),
		180*time.Second,
		total,
	))
}
