package replication

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/transferia/transferia/internal/logger"
	"github.com/transferia/transferia/library/go/core/metrics/solomon"
	"github.com/transferia/transferia/pkg/abstract"
	cpclient "github.com/transferia/transferia/pkg/abstract/coordinator"
	"github.com/transferia/transferia/pkg/abstract/model"
	"github.com/transferia/transferia/pkg/parsers"
	jsonparser "github.com/transferia/transferia/pkg/parsers/registry/json"
	chrecipe "github.com/transferia/transferia/pkg/providers/clickhouse/recipe"
	kafkasink "github.com/transferia/transferia/pkg/providers/kafka"
	"github.com/transferia/transferia/tests/helpers"
	ytschema "go.ytsaurus.tech/yt/go/schema"
)

func parserConfig(t *testing.T) map[string]interface{} {
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

func kafkaSink(t *testing.T, src *kafkasink.KafkaSource) abstract.Sinker {
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

func pushMessage(t *testing.T, sink abstract.Sinker, topic string, offset int64, payload map[string]any) {
	t.Helper()
	v, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, sink.Push([]abstract.ChangeItem{
		abstract.MakeRawMessage([]byte("_"), topic, time.Time{}, topic, 0, offset, v),
	}))
}

func TestResumeFromCoordinator(t *testing.T) {
	source := *kafkasink.MustSourceRecipe()
	source.Topic = fmt.Sprintf("kafka_resume_%d", time.Now().UnixNano())
	source.ParserConfig = parserConfig(t)

	target := *chrecipe.MustTarget(chrecipe.WithInitDir("dump/ch"), chrecipe.WithDatabase("public"))

	transfer := helpers.MakeTransfer(helpers.GenerateTransferID(t.Name()), &source, &target, abstract.TransferTypeIncrementOnly)
	cp := cpclient.NewStatefulFakeClient()

	worker1, err := helpers.ActivateWithCP(transfer, cp, true)
	require.NoError(t, err)

	sink := kafkaSink(t, &source)
	for i := 0; i < 5; i++ {
		pushMessage(t, sink, source.Topic, int64(i), map[string]any{"id": i + 1, "msg": fmt.Sprintf("first_%d", i+1)})
	}

	require.NoError(t, helpers.WaitDestinationEqualRowsCount(
		"public",
		source.Topic,
		helpers.GetSampleableStorageByModel(t, target),
		90*time.Second,
		5,
	))

	worker1.Close(t)

	worker2, err := helpers.ActivateWithCP(transfer, cp, true)
	require.NoError(t, err)
	defer worker2.Close(t)

	require.NoError(t, helpers.WaitDestinationEqualRowsCount(
		"public",
		source.Topic,
		helpers.GetSampleableStorageByModel(t, target),
		90*time.Second,
		5,
	))

	for i := 5; i < 8; i++ {
		pushMessage(t, sink, source.Topic, int64(i), map[string]any{"id": i + 1, "msg": fmt.Sprintf("second_%d", i+1)})
	}

	require.NoError(t, helpers.WaitDestinationEqualRowsCount(
		"public",
		source.Topic,
		helpers.GetSampleableStorageByModel(t, target),
		90*time.Second,
		8,
	))
}
