package json

import (
	"github.com/transferia/transferia/pkg/abstract"
)

type ParserConfigJSONLb struct {
	// common parameters - for lb/kafka/yds/eventhub
	Fields             []abstract.ColSchema
	SchemaResourceName string // for the case, when logfeller-schema-id. Only for internal installation!

	NullKeysAllowed      bool // (title: "Использовать значение NULL в ключевых столбцах", "Разрешить NULL в ключевых колонках")
	AddRest              bool // (title: "Добавить неразмеченные столбцы", usage: "Поля, отсутствующие в схеме, попадут в колонку _rest")
	UnescapeStringValues bool

	// special parameters for logbroker-source:
	SkipSystemKeys    bool                   // aka skip_dedupe_keys/SkipDedupeKeys (title: "Пользовательские ключевые столбцы", usage: "При парсинге ключи дедубликации Logbroker не будут добавлены к списку пользовательских ключевых столбцов")
	TimeField         *abstract.TimestampCol // (title: "Столбец, содержащий дату-время")
	AddSystemCols     bool                   // (title: "Добавление системных столбцов Logbroker", usage: "CreateTime (_lb_ctime) WriteTime (_lb_wtime) и все Headers с префиксом _lb_extra_")
	TableSplitter     *abstract.TableSplitter
	IgnoreColumnPaths bool
	DropUnparsed      bool
	MaskSecrets       bool

	// private option
	UseNumbersInAny bool
}

func (c *ParserConfigJSONLb) IsNewParserConfig() {}

func (c *ParserConfigJSONLb) IsAppendOnly() bool {
	return true
}

func (c *ParserConfigJSONLb) Validate() error {
	return nil
}
