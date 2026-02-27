package mysql

import (
	"regexp"
	"strings"

	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
)

var utf8mb3Pattern = regexp.MustCompile(`(?i)utf8mb3`)

func parseWithCharsetCompat(p *parser.Parser, ddl string) ([]ast.StmtNode, []error, error) {
	stmts, warns, err := p.Parse(ddl, "", "")
	if err == nil {
		return stmts, warns, nil
	}

	if !isUnknownUTF8MB3(err) {
		return nil, nil, err
	}

	normalizedDDL := utf8mb3Pattern.ReplaceAllString(ddl, "utf8")
	return p.Parse(normalizedDDL, "", "")
}

func isUnknownUTF8MB3(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown character set") && strings.Contains(msg, "utf8mb3")
}
