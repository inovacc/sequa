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
	CmdOne        QueryCmd = ":one"
	CmdMany       QueryCmd = ":many"
	CmdExec       QueryCmd = ":exec"
	CmdExecRows   QueryCmd = ":execrows"   // Exec returning rows-affected (int64)
	CmdExecResult QueryCmd = ":execresult" // Exec returning sql.Result
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

var (
	// queryHeaderRe matches a well-formed query header: `-- name: Name :verb`.
	// It accepts ANY :verb (validated by validateVerb) so an unknown verb is
	// reported clearly rather than being silently merged into the previous
	// query's SQL.
	queryHeaderRe = regexp.MustCompile(`^--\s*name:\s+(\w+)\s+(:\w+)\s*$`)
	// nameLineRe matches a header-SHAPED line (`-- name: Foo :verb`, including a
	// stray space before the colon or trailing junk) that queryHeaderRe rejected,
	// so a real header typo fails loudly — while a prose comment that merely
	// starts with "-- name:" (e.g. "-- name: lookup is by primary key") stays
	// ordinary SQL.
	nameLineRe = regexp.MustCompile(`^--\s*name\s*:\s+\w+\s+:\w+`)
)

// isExecFamily reports whether cmd is an exec-style verb that scans no result
// columns.
func isExecFamily(cmd QueryCmd) bool {
	switch cmd {
	case CmdExec, CmdExecRows, CmdExecResult:
		return true
	default:
		return false
	}
}

// validateVerb rejects unknown or unsupported query verbs with an actionable
// error, so a bad annotation fails clearly at generation time instead of
// producing broken or misleading output.
func validateVerb(cmd QueryCmd) error {
	switch cmd {
	case CmdOne, CmdMany, CmdExec, CmdExecRows, CmdExecResult:
		return nil
	case ":copyfrom", ":batchexec", ":batchmany", ":batchone":
		return fmt.Errorf("unsupported command %q: requires the pgx driver (sequa generates database/sql + lib/pq code)", cmd)
	case ":execlastid":
		return fmt.Errorf("unsupported command %q: LastInsertId is not available on PostgreSQL — use RETURNING with :one", cmd)
	default:
		return fmt.Errorf("unknown command %q (want :one, :many, :exec, :execrows, or :execresult)", cmd)
	}
}

func parseQueryFile(content string) ([]rawQuery, error) {
	var out []rawQuery
	var cur *rawQuery
	flush := func() {
		if cur != nil {
			cur.SQL = strings.TrimSpace(cur.SQL)
			out = append(out, *cur)
		}
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := queryHeaderRe.FindStringSubmatch(trimmed); m != nil {
			flush()
			cur = &rawQuery{Name: m[1], Cmd: QueryCmd(m[2])}
			continue
		}
		if nameLineRe.MatchString(trimmed) {
			return nil, fmt.Errorf("malformed query header %q (want `-- name: Name :verb`)", trimmed)
		}
		if cur != nil {
			cur.SQL += line + "\n"
		}
	}
	flush()
	return out, nil
}

// AnalyzeQueries parses a queries file and types each query against the catalog.
func AnalyzeQueries(cat *Catalog, content string) ([]Query, error) {
	raws, err := parseQueryFile(content)
	if err != nil {
		return nil, err
	}
	var queries []Query
	for _, rq := range raws {
		q, err := analyzeQuery(cat, rq)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", rq.Name, err)
		}
		queries = append(queries, q)
	}
	return queries, nil
}

func analyzeQuery(cat *Catalog, rq rawQuery) (Query, error) {
	if err := validateVerb(rq.Cmd); err != nil {
		return Query{}, err
	}
	res, err := pgquery.Parse(rq.SQL)
	if err != nil {
		return Query{}, fmt.Errorf("parse: %w", err)
	}
	if len(res.Stmts) != 1 {
		return Query{}, fmt.Errorf("expected exactly one SQL statement")
	}
	q := Query{Name: rq.Name, Cmd: rq.Cmd, SQL: rq.SQL}
	binds := map[int]Column{} // param number -> column it binds to
	if err := q.bindStatement(cat, res.Stmts[0].Stmt, binds); err != nil {
		return Query{}, err
	}
	params, err := inferParams(binds)
	if err != nil {
		return Query{}, err
	}
	q.Params = params

	if isExecFamily(q.Cmd) {
		q.Columns, q.Star = nil, false
	} else if len(q.Columns) == 0 {
		return Query{}, fmt.Errorf("%s query returns no columns", q.Cmd)
	}
	return q, nil
}

// bindStatement walks the parsed statement, recording parameter bindings into
// binds and the result columns onto q. Only single-table SELECT/INSERT/UPDATE/
// DELETE are supported.
func (q *Query) bindStatement(cat *Catalog, stmt *pgquery.Node, binds map[int]Column) error {
	switch {
	case stmt.GetSelectStmt() != nil:
		return q.bindSelect(cat, stmt.GetSelectStmt(), binds)
	case stmt.GetInsertStmt() != nil:
		s := stmt.GetInsertStmt()
		q.Table = relName(s.Relation)
		bindInsert(cat, q.Table, s, binds)
		return q.setResults(cat, q.Table, s.ReturningList)
	case stmt.GetUpdateStmt() != nil:
		s := stmt.GetUpdateStmt()
		q.Table = relName(s.Relation)
		bindUpdate(cat, q.Table, s, binds)
		bindWhere(cat, q.Table, s.WhereClause, binds)
		return q.setResults(cat, q.Table, s.ReturningList)
	case stmt.GetDeleteStmt() != nil:
		s := stmt.GetDeleteStmt()
		q.Table = relName(s.Relation)
		bindWhere(cat, q.Table, s.WhereClause, binds)
		return q.setResults(cat, q.Table, s.ReturningList)
	default:
		return fmt.Errorf("unsupported statement type (SELECT/INSERT/UPDATE/DELETE only)")
	}
}

// inferParams turns the param->column bindings into ordered, uniquely-named
// typed parameters ($1..$N). A gap in the numbering means a parameter's type
// could not be inferred.
func inferParams(binds map[int]Column) ([]Param, error) {
	maxN := 0
	for n := range binds {
		if n > maxN {
			maxN = n
		}
	}
	// Seed with the identifiers the generated method bodies use — the receiver
	// (q), ctx, and the per-verb locals — so a parameter bound to a column of the
	// same name is renamed (e.g. result -> result2) instead of shadowing or
	// redeclaring them, which passes gofmt but fails to compile.
	used := map[string]int{
		"ctx": 1, "q": 1, "err": 1, "result": 1,
		"row": 1, "rows": 1, "i": 1, "items": 1,
	}
	var params []Param
	for n := 1; n <= maxN; n++ {
		col, ok := binds[n]
		if !ok {
			return nil, fmt.Errorf("could not infer the type of parameter $%d", n)
		}
		params = append(params, Param{Number: n, Name: argName(col.Name, used), GoType: goTypeFor(col)})
	}
	return params, nil
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

// relation is one FROM/JOIN table together with the name (its alias, else the
// table name) that qualified column references (t.col) resolve against.
type relation struct {
	name  string
	table *Table
}

// bindSelect binds a SELECT. A single-table (or no-FROM) select preserves the
// original single-table typing exactly; a multi-table (JOIN) select resolves
// result columns and parameters across the joined relations.
func (q *Query) bindSelect(cat *Catalog, s *pgquery.SelectStmt, binds map[int]Column) error {
	rels, err := collectRelations(cat, s.FromClause)
	if err != nil {
		return err
	}
	if len(rels) <= 1 {
		table := ""
		if len(rels) == 1 {
			table = rels[0].table.Name
		}
		q.Table = table
		bindWhere(cat, table, s.WhereClause, binds)
		return q.setResults(cat, table, s.TargetList)
	}
	return q.bindJoinSelect(rels, s, binds)
}

// bindJoinSelect types a JOIN select: params bind across the relations and the
// result is an explicit column list scanned into a per-query row struct (never
// a table's model), so q.Star is always false.
func (q *Query) bindJoinSelect(rels []relation, s *pgquery.SelectStmt, binds map[int]Column) error {
	bindWhereRel(rels, s.WhereClause, binds)
	cols, err := resolveJoinTargets(rels, s.TargetList)
	if err != nil {
		return err
	}
	q.Columns, q.Star, q.Table = cols, false, ""
	return nil
}

// collectRelations walks a SELECT's FROM clause into an ordered list of
// relations, descending INNER JOIN trees. Outer joins and non-table FROM items
// (subqueries, functions) are rejected.
func collectRelations(cat *Catalog, from []*pgquery.Node) ([]relation, error) {
	var rels []relation
	for _, n := range from {
		got, err := collectFromNode(cat, n)
		if err != nil {
			return nil, err
		}
		rels = append(rels, got...)
	}
	return rels, nil
}

func collectFromNode(cat *Catalog, n *pgquery.Node) ([]relation, error) {
	if rv := n.GetRangeVar(); rv != nil {
		r, err := relationFromRangeVar(cat, rv)
		if err != nil {
			return nil, err
		}
		return []relation{r}, nil
	}
	if je := n.GetJoinExpr(); je != nil {
		return collectJoin(cat, je)
	}
	return nil, fmt.Errorf("unsupported FROM item (only tables and INNER JOINs are supported)")
}

func collectJoin(cat *Catalog, je *pgquery.JoinExpr) ([]relation, error) {
	if je.GetJointype() != pgquery.JoinType_JOIN_INNER {
		return nil, fmt.Errorf("only INNER JOIN is supported; outer joins are planned")
	}
	left, err := collectFromNode(cat, je.GetLarg())
	if err != nil {
		return nil, err
	}
	right, err := collectFromNode(cat, je.GetRarg())
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

func relationFromRangeVar(cat *Catalog, rv *pgquery.RangeVar) (relation, error) {
	t := cat.Table(rv.Relname)
	if t == nil {
		return relation{}, fmt.Errorf("unknown table %q", rv.Relname)
	}
	name := rv.Relname
	if rv.Alias != nil && rv.Alias.Aliasname != "" {
		name = rv.Alias.Aliasname
	}
	return relation{name: name, table: t}, nil
}

// columnRefParts splits a ColumnRef into an optional qualifier (t in t.col) and
// the column name. ok is false for a non-column ref such as "*" or "t.*", whose
// last field is an A_Star rather than a String_.
func columnRefParts(node *pgquery.Node) (qualifier, column string, ok bool) {
	if node == nil {
		return "", "", false
	}
	cr := node.GetColumnRef()
	if cr == nil || len(cr.Fields) == 0 {
		return "", "", false
	}
	last := cr.Fields[len(cr.Fields)-1].GetString_()
	if last == nil {
		return "", "", false
	}
	if len(cr.Fields) >= 2 {
		if q := cr.Fields[len(cr.Fields)-2].GetString_(); q != nil {
			qualifier = q.Sval
		}
	}
	return qualifier, last.Sval, true
}

func columnRefName(node *pgquery.Node) (string, bool) {
	_, col, ok := columnRefParts(node)
	return col, ok
}

// resolveRelationColumn resolves a (possibly qualified) column reference against
// the joined relations. An unqualified name present in more than one relation is
// ambiguous; a name in none is unknown.
func resolveRelationColumn(rels []relation, qualifier, col string) (Column, error) {
	if qualifier != "" {
		return resolveQualified(rels, qualifier, col)
	}
	var found Column
	matches := 0
	for _, r := range rels {
		if c, ok := findColumn(r.table, col); ok {
			found, matches = c, matches+1
		}
	}
	switch matches {
	case 0:
		return Column{}, fmt.Errorf("unknown column %q in the joined tables", col)
	case 1:
		return found, nil
	default:
		return Column{}, fmt.Errorf("ambiguous column %q; qualify it with a table or alias", col)
	}
}

func resolveQualified(rels []relation, qualifier, col string) (Column, error) {
	for _, r := range rels {
		if r.name != qualifier {
			continue
		}
		if c, ok := findColumn(r.table, col); ok {
			return c, nil
		}
		return Column{}, fmt.Errorf("unknown column %q in %q", col, qualifier)
	}
	return Column{}, fmt.Errorf("unknown table or alias %q", qualifier)
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

// bindWhereRel is the JOIN-aware counterpart of bindWhere: it binds params in a
// WHERE clause to columns resolved across the joined relations.
func bindWhereRel(rels []relation, where *pgquery.Node, binds map[int]Column) {
	if where == nil {
		return
	}
	if be := where.GetBoolExpr(); be != nil {
		for _, arg := range be.Args {
			bindWhereRel(rels, arg, binds)
		}
		return
	}
	if ae := where.GetAExpr(); ae != nil {
		bindAExprRel(rels, ae, binds)
	}
}

func bindAExprRel(rels []relation, ae *pgquery.A_Expr, binds map[int]Column) {
	lq, lcol, lok := columnRefParts(ae.Lexpr)
	rq, rcol, rok := columnRefParts(ae.Rexpr)
	if lok && paramNum(ae.Rexpr) > 0 {
		bindColRel(rels, lq, lcol, paramNum(ae.Rexpr), binds)
	} else if rok && paramNum(ae.Lexpr) > 0 {
		bindColRel(rels, rq, rcol, paramNum(ae.Lexpr), binds)
	}
	bindWhereRel(rels, ae.Lexpr, binds)
	bindWhereRel(rels, ae.Rexpr, binds)
}

// bindColRel binds param n to a (possibly qualified) column when it resolves
// unambiguously. Resolution failures are skipped silently (mirroring the
// single-table binder) so a missing binding surfaces later in param typing.
func bindColRel(rels []relation, qualifier, col string, n int, binds map[int]Column) {
	if c, err := resolveRelationColumn(rels, qualifier, col); err == nil {
		binds[n] = c
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

// resolveJoinTargets types every explicit result column of a JOIN query. It
// rejects "*" (single-table "*" keeps its own fast path) and duplicate result
// names, which would otherwise produce colliding Go struct fields.
func resolveJoinTargets(rels []relation, targets []*pgquery.Node) ([]Column, error) {
	var cols []Column
	seen := map[string]bool{}
	for _, tg := range targets {
		rt := tg.GetResTarget()
		if rt == nil {
			continue
		}
		col, err := resolveJoinResultColumn(rels, rt)
		if err != nil {
			return nil, err
		}
		// Key on the generated Go field name: distinct SQL names can still fold
		// to the same identifier (e.g. "ID" and "id" both -> ID).
		field := goName(col.Name)
		if seen[field] {
			return nil, fmt.Errorf("result column %q collides with another result field %q; alias one with AS", col.Name, field)
		}
		seen[field] = true
		cols = append(cols, col)
	}
	return cols, nil
}

// resolveJoinResultColumn types one result target of a JOIN query: a qualified
// or unqualified column, or a supported aggregate over a joined column. When an
// AS alias is present it names the result column (so duplicate bare names can be
// disambiguated).
func resolveJoinResultColumn(rels []relation, rt *pgquery.ResTarget) (Column, error) {
	if isStar(rt.Val) {
		return Column{}, fmt.Errorf("SELECT * is not supported with JOINs; list columns explicitly")
	}
	if qualifier, col, ok := columnRefParts(rt.Val); ok {
		resolved, err := resolveRelationColumn(rels, qualifier, col)
		if err != nil {
			return Column{}, err
		}
		if rt.Name != "" {
			resolved.Name = rt.Name
		}
		return resolved, nil
	}
	if fc := rt.Val.GetFuncCall(); fc != nil {
		return aggregateColumnRel(rels, rt, fc)
	}
	return Column{}, fmt.Errorf("unsupported result expression in JOIN query (plain columns and count/min/max/sum/avg aggregates only)")
}

// aggName is the result-column name of an aggregate: its AS alias, or the
// function name when none is given.
func aggName(rt *pgquery.ResTarget, fn string) string {
	if rt.Name != "" {
		return rt.Name
	}
	return fn
}

// countColumn is the non-null bigint result of count(*)/count(expr).
func countColumn(name string) Column {
	return Column{Name: name, PgType: "int8", NotNull: true}
}

// aggregateColumn types a single-table count/min/max/sum/avg aggregate.
func aggregateColumn(t *Table, rt *pgquery.ResTarget, fc *pgquery.FuncCall) (Column, error) {
	fn := funcName(fc)
	name := aggName(rt, fn)
	if fn == "count" {
		return countColumn(name), nil
	}
	arg, ok := singleColumnArg(fc)
	if !ok {
		return Column{}, fmt.Errorf("%s(...) must take a single plain column argument", fn)
	}
	col, found := findColumn(t, arg)
	if !found {
		return Column{}, fmt.Errorf("unknown column %q in %s(...)", arg, fn)
	}
	return aggregateResult(fn, name, col)
}

// aggregateColumnRel types an aggregate whose argument column is resolved across
// the joined relations.
func aggregateColumnRel(rels []relation, rt *pgquery.ResTarget, fc *pgquery.FuncCall) (Column, error) {
	fn := funcName(fc)
	name := aggName(rt, fn)
	if fn == "count" {
		return countColumn(name), nil
	}
	qualifier, arg, ok := singleColumnArgParts(fc)
	if !ok {
		return Column{}, fmt.Errorf("%s(...) must take a single plain column argument", fn)
	}
	col, err := resolveRelationColumn(rels, qualifier, arg)
	if err != nil {
		return Column{}, err
	}
	return aggregateResult(fn, name, col)
}

// aggregateResult types min/max/sum/avg over an already-resolved argument
// column. min/max keep the column's type but may be NULL over an empty set;
// sum/avg follow Postgres's numeric promotion. count is handled by countColumn.
func aggregateResult(fn, name string, arg Column) (Column, error) {
	switch fn {
	case "min", "max":
		return Column{Name: name, PgType: arg.PgType, NotNull: false, Array: arg.Array}, nil
	case "sum", "avg":
		pgType, ok := numericAggregateType(fn, arg.PgType)
		if !ok {
			return Column{}, fmt.Errorf("%s(%s) is unsupported (numeric column types only)", fn, arg.PgType)
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

// singleColumnArgParts returns the (optional qualifier, column) of fc when it
// has exactly one plain column-reference argument.
func singleColumnArgParts(fc *pgquery.FuncCall) (qualifier, column string, ok bool) {
	if len(fc.Args) != 1 {
		return "", "", false
	}
	return columnRefParts(fc.Args[0])
}

// singleColumnArg returns the column name when fc has exactly one plain
// column-reference argument.
func singleColumnArg(fc *pgquery.FuncCall) (string, bool) {
	_, col, ok := singleColumnArgParts(fc)
	return col, ok
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
