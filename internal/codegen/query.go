package codegen

import (
	"fmt"
	"regexp"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v5"
)

// QueryCmd is the sqlc-style command annotation on a query.
type QueryCmd string

const (
	CmdOne  QueryCmd = ":one"
	CmdMany QueryCmd = ":many"
	CmdExec QueryCmd = ":exec"
)

// Param is a query parameter ($N) with its inferred Go type.
type Param struct {
	Number int
	Name   string
	GoType GoType
}

// Query is an analyzed annotated query.
type Query struct {
	Name    string
	Cmd     QueryCmd
	SQL     string
	Params  []Param
	Columns []Column // result columns (empty for :exec)
	Star    bool     // result is "*" / "RETURNING *" of a single table
	Table   string   // table the result columns come from
}

type rawQuery struct {
	Name string
	Cmd  QueryCmd
	SQL  string
}

var queryHeaderRe = regexp.MustCompile(`^--\s*name:\s+(\w+)\s+(:one|:many|:exec)\s*$`)

func parseQueryFile(content string) []rawQuery {
	var out []rawQuery
	var cur *rawQuery
	flush := func() {
		if cur != nil {
			cur.SQL = strings.TrimSpace(cur.SQL)
			out = append(out, *cur)
		}
	}
	for _, line := range strings.Split(content, "\n") {
		if m := queryHeaderRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			flush()
			cur = &rawQuery{Name: m[1], Cmd: QueryCmd(m[2])}
			continue
		}
		if cur != nil {
			cur.SQL += line + "\n"
		}
	}
	flush()
	return out
}

// AnalyzeQueries parses a queries file and types each query against the catalog.
func AnalyzeQueries(cat *Catalog, content string) ([]Query, error) {
	var queries []Query
	for _, rq := range parseQueryFile(content) {
		q, err := analyzeQuery(cat, rq)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", rq.Name, err)
		}
		queries = append(queries, q)
	}
	return queries, nil
}

func analyzeQuery(cat *Catalog, rq rawQuery) (Query, error) {
	res, err := pgquery.Parse(rq.SQL)
	if err != nil {
		return Query{}, fmt.Errorf("parse: %w", err)
	}
	if len(res.Stmts) != 1 {
		return Query{}, fmt.Errorf("expected exactly one SQL statement")
	}
	q := Query{Name: rq.Name, Cmd: rq.Cmd, SQL: rq.SQL}
	stmt := res.Stmts[0].Stmt

	binds := map[int]Column{} // param number -> column it binds to

	switch {
	case stmt.GetSelectStmt() != nil:
		s := stmt.GetSelectStmt()
		q.Table = fromTable(s.FromClause)
		bindWhere(cat, q.Table, s.WhereClause, binds)
		if err := q.setResults(cat, q.Table, s.TargetList); err != nil {
			return Query{}, err
		}
	case stmt.GetInsertStmt() != nil:
		s := stmt.GetInsertStmt()
		q.Table = relName(s.Relation)
		bindInsert(cat, q.Table, s, binds)
		if err := q.setResults(cat, q.Table, s.ReturningList); err != nil {
			return Query{}, err
		}
	case stmt.GetUpdateStmt() != nil:
		s := stmt.GetUpdateStmt()
		q.Table = relName(s.Relation)
		bindUpdate(cat, q.Table, s, binds)
		bindWhere(cat, q.Table, s.WhereClause, binds)
		if err := q.setResults(cat, q.Table, s.ReturningList); err != nil {
			return Query{}, err
		}
	case stmt.GetDeleteStmt() != nil:
		s := stmt.GetDeleteStmt()
		q.Table = relName(s.Relation)
		bindWhere(cat, q.Table, s.WhereClause, binds)
		if err := q.setResults(cat, q.Table, s.ReturningList); err != nil {
			return Query{}, err
		}
	default:
		return Query{}, fmt.Errorf("unsupported statement type (SELECT/INSERT/UPDATE/DELETE only)")
	}

	maxN := 0
	for n := range binds {
		if n > maxN {
			maxN = n
		}
	}
	used := map[string]int{}
	for n := 1; n <= maxN; n++ {
		col, ok := binds[n]
		if !ok {
			return Query{}, fmt.Errorf("could not infer the type of parameter $%d", n)
		}
		q.Params = append(q.Params, Param{Number: n, Name: argName(col.Name, used), GoType: goTypeFor(col)})
	}

	if q.Cmd == CmdExec {
		q.Columns, q.Star = nil, false
	} else if len(q.Columns) == 0 {
		return Query{}, fmt.Errorf("%s query returns no columns", q.Cmd)
	}
	return q, nil
}

func (q *Query) setResults(cat *Catalog, table string, targets []*pgquery.Node) error {
	cols, star, err := resolveTargets(cat, table, targets)
	if err != nil {
		return err
	}
	q.Columns, q.Star = cols, star
	return nil
}

func relName(r *pgquery.RangeVar) string {
	if r == nil {
		return ""
	}
	return r.Relname
}

func fromTable(from []*pgquery.Node) string {
	for _, n := range from {
		if rv := n.GetRangeVar(); rv != nil {
			return rv.Relname
		}
	}
	return ""
}

func columnRefName(node *pgquery.Node) (string, bool) {
	if node == nil {
		return "", false
	}
	cr := node.GetColumnRef()
	if cr == nil || len(cr.Fields) == 0 {
		return "", false
	}
	last := cr.Fields[len(cr.Fields)-1]
	if s := last.GetString_(); s != nil {
		return s.Sval, true
	}
	return "", false
}

func paramNum(node *pgquery.Node) int {
	if node == nil {
		return 0
	}
	if p := node.GetParamRef(); p != nil {
		return int(p.Number)
	}
	return 0
}

func bindCol(cat *Catalog, table, col string, n int, binds map[int]Column) {
	t := cat.Table(table)
	if t == nil {
		return
	}
	if c, ok := findColumn(t, col); ok {
		binds[n] = c
	}
}

func bindWhere(cat *Catalog, table string, where *pgquery.Node, binds map[int]Column) {
	if where == nil {
		return
	}
	if be := where.GetBoolExpr(); be != nil {
		for _, arg := range be.Args {
			bindWhere(cat, table, arg, binds)
		}
		return
	}
	if ae := where.GetAExpr(); ae != nil {
		lcol, lok := columnRefName(ae.Lexpr)
		rcol, rok := columnRefName(ae.Rexpr)
		if lok && paramNum(ae.Rexpr) > 0 {
			bindCol(cat, table, lcol, paramNum(ae.Rexpr), binds)
		} else if rok && paramNum(ae.Lexpr) > 0 {
			bindCol(cat, table, rcol, paramNum(ae.Lexpr), binds)
		}
		bindWhere(cat, table, ae.Lexpr, binds)
		bindWhere(cat, table, ae.Rexpr, binds)
	}
}

func bindInsert(cat *Catalog, table string, s *pgquery.InsertStmt, binds map[int]Column) {
	var cols []string
	for _, c := range s.Cols {
		if rt := c.GetResTarget(); rt != nil {
			cols = append(cols, rt.Name)
		}
	}
	sel := s.SelectStmt.GetSelectStmt()
	if sel == nil || len(sel.ValuesLists) == 0 {
		return
	}
	values := sel.ValuesLists[0].GetList()
	if values == nil {
		return
	}
	for i, v := range values.Items {
		if i >= len(cols) {
			break
		}
		if n := paramNum(v); n > 0 {
			bindCol(cat, table, cols[i], n, binds)
		}
	}
}

func bindUpdate(cat *Catalog, table string, s *pgquery.UpdateStmt, binds map[int]Column) {
	for _, t := range s.TargetList {
		rt := t.GetResTarget()
		if rt == nil {
			continue
		}
		if n := paramNum(rt.Val); n > 0 {
			bindCol(cat, table, rt.Name, n, binds)
		}
	}
}

func resolveTargets(cat *Catalog, table string, targets []*pgquery.Node) ([]Column, bool, error) {
	if len(targets) == 0 {
		return nil, false, nil
	}
	t := cat.Table(table)
	if t == nil {
		return nil, false, fmt.Errorf("unknown table %q", table)
	}
	if len(targets) == 1 {
		if rt := targets[0].GetResTarget(); rt != nil && isStar(rt.Val) {
			return append([]Column(nil), t.Columns...), true, nil
		}
	}
	var cols []Column
	for _, tg := range targets {
		rt := tg.GetResTarget()
		if rt == nil {
			continue
		}
		col, err := resolveResultColumn(t, rt)
		if err != nil {
			return nil, false, err
		}
		cols = append(cols, col)
	}
	return cols, false, nil
}

// resolveResultColumn types one SELECT/RETURNING target: a plain column
// reference or a supported aggregate (count/min/max).
func resolveResultColumn(t *Table, rt *pgquery.ResTarget) (Column, error) {
	if name, ok := columnRefName(rt.Val); ok {
		col, found := findColumn(t, name)
		if !found {
			return Column{}, fmt.Errorf("unknown column %q in table %q", name, t.Name)
		}
		return col, nil
	}
	if fc := rt.Val.GetFuncCall(); fc != nil {
		return aggregateColumn(t, rt, fc)
	}
	return Column{}, fmt.Errorf("unsupported result expression (plain columns, *, and count/min/max aggregates are supported)")
}

// aggregateColumn types a count/min/max aggregate. count returns a non-null
// bigint; min/max keep the argument column's type but may be NULL over an empty
// set. The result column is named by its alias, or by the function name if none.
func aggregateColumn(t *Table, rt *pgquery.ResTarget, fc *pgquery.FuncCall) (Column, error) {
	fn := funcName(fc)
	name := rt.Name
	if name == "" {
		name = fn
	}
	if fn == "count" {
		// count(*) and count(expr) both return a non-null bigint.
		return Column{Name: name, PgType: "int8", NotNull: true}, nil
	}
	arg, ok := singleColumnArg(fc)
	if !ok {
		return Column{}, fmt.Errorf("%s(...) must take a single plain column argument", fn)
	}
	col, found := findColumn(t, arg)
	if !found {
		return Column{}, fmt.Errorf("unknown column %q in %s(...)", arg, fn)
	}
	switch fn {
	case "min", "max":
		// min/max preserve the column's type but can be NULL over an empty set.
		return Column{Name: name, PgType: col.PgType, NotNull: false, Array: col.Array}, nil
	case "sum", "avg":
		pgType, ok := numericAggregateType(fn, col.PgType)
		if !ok {
			return Column{}, fmt.Errorf("%s(%s) is unsupported (numeric column types only)", fn, col.PgType)
		}
		return Column{Name: name, PgType: pgType, NotNull: false}, nil
	default:
		return Column{}, fmt.Errorf("unsupported aggregate %q (count/min/max/sum/avg only)", fn)
	}
}

// numericAggregateType returns the Postgres result type of sum/avg over a
// column of argPg, following Postgres's own promotion rules: sum of a small
// integer widens to bigint, sum of bigint/numeric is numeric, avg of any exact
// type is numeric, and floating types stay in the float family.
func numericAggregateType(fn, argPg string) (string, bool) {
	switch fn {
	case "sum":
		switch argPg {
		case "int2", "int4":
			return "int8", true
		case "int8", "numeric":
			return "numeric", true
		case "float4":
			return "float4", true
		case "float8":
			return "float8", true
		}
	case "avg":
		switch argPg {
		case "int2", "int4", "int8", "numeric":
			return "numeric", true
		case "float4", "float8":
			return "float8", true
		}
	}
	return "", false
}

// funcName returns the lower-cased final element of a function's name.
func funcName(fc *pgquery.FuncCall) string {
	if len(fc.Funcname) == 0 {
		return ""
	}
	if s := fc.Funcname[len(fc.Funcname)-1].GetString_(); s != nil {
		return strings.ToLower(s.Sval)
	}
	return ""
}

// singleColumnArg returns the column name when fc has exactly one plain
// column-reference argument.
func singleColumnArg(fc *pgquery.FuncCall) (string, bool) {
	if len(fc.Args) != 1 {
		return "", false
	}
	return columnRefName(fc.Args[0])
}

func isStar(node *pgquery.Node) bool {
	if node == nil {
		return false
	}
	cr := node.GetColumnRef()
	if cr == nil || len(cr.Fields) == 0 {
		return false
	}
	return cr.Fields[len(cr.Fields)-1].GetAStar() != nil
}

func findColumn(t *Table, name string) (Column, bool) {
	for _, c := range t.Columns {
		if c.Name == name {
			return c, true
		}
	}
	return Column{}, false
}

func argName(col string, used map[string]int) string {
	base := lowerCamel(col)
	if base == "" {
		base = "arg"
	}
	used[base]++
	if used[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s%d", base, used[base])
}
