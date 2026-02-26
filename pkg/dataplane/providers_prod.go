package dataplane

import (
	_ "github.com/transferia/transferia/pkg/providers/airbyte"
	_ "github.com/transferia/transferia/pkg/providers/clickhouse"
	_ "github.com/transferia/transferia/pkg/providers/eventhub"
	_ "github.com/transferia/transferia/pkg/providers/kafka"
	_ "github.com/transferia/transferia/pkg/providers/kinesis"
	_ "github.com/transferia/transferia/pkg/providers/mongo"
	_ "github.com/transferia/transferia/pkg/providers/mysql"
	_ "github.com/transferia/transferia/pkg/providers/oracle"
	_ "github.com/transferia/transferia/pkg/providers/postgres"
	_ "github.com/transferia/transferia/pkg/providers/sample"
	_ "github.com/transferia/transferia/pkg/providers/stdout"
)
