// Package codegen builds a schema catalog from migration DDL and renders
// idiomatic Go (models and typed query methods) from it — the engine behind
// sequa's `generate`. Schema is parsed statically with PostgreSQL's own
// parser (pg_query_go / libpg_query), no database required.
package codegen

import (
	"fmt"
	"slices"

	pgquery "github.com/pganalyze/pg_query_go/v5"
)

// Column is a table column derived from DDL.
type Column struct {
	Name    string
	PgType  string // normalized pg type name, e.g. int8, text, timestamptz, bool
	NotNull bool
	Array   bool
}

// Table is a database table and its columns, in definition order.
type Table struct {
	Name    string
	Columns []Column
}

// Catalog is the schema assembled by applying the ordered up-migrations.
type Catalog struct {
	Tables []*Table // creation order
	byName map[string]*Table
}

func newCatalog() *Catalog { return &Catalog{byName: map[string]*Table{}} }

// Table returns a table by name, or nil if it does not exist.
func (c *Catalog) Table(name string) *Table { return c.byName[name] }

// BuildCatalog parses each migration's SQL (in order) and applies its DDL —
// CREATE TABLE, ALTER TABLE ADD/DROP COLUMN, DROP TABLE — to assemble the
// schema the queries will be checked against.
func BuildCatalog(migrations []string) (*Catalog, error) {
	cat := newCatalog()
	for i, sql := range migrations {
		result, err := pgquery.Parse(sql)
		if err != nil {
			return nil, fmt.Errorf("parse migration #%d: %w", i+1, err)
		}
		for _, raw := range result.Stmts {
			cat.apply(raw.Stmt)
		}
	}
	return cat, nil
}

func (c *Catalog) apply(node *pgquery.Node) {
	if node == nil {
		return
	}
	if s := node.GetCreateStmt(); s != nil {
		c.applyCreate(s)
		return
	}
	if s := node.GetAlterTableStmt(); s != nil {
		c.applyAlter(s)
		return
	}
	if s := node.GetDropStmt(); s != nil {
		c.applyDrop(s)
	}
}

func (c *Catalog) applyCreate(s *pgquery.CreateStmt) {
	if s.Relation == nil {
		return
	}
	t := &Table{Name: s.Relation.Relname}
	for _, elt := range s.TableElts {
		if col := elt.GetColumnDef(); col != nil {
			t.Columns = append(t.Columns, columnFromDef(col))
		}
	}
	if _, exists := c.byName[t.Name]; !exists {
		c.Tables = append(c.Tables, t)
	}
	c.byName[t.Name] = t
}

func columnFromDef(col *pgquery.ColumnDef) Column {
	c := Column{Name: col.Colname, PgType: typeNameOf(col.TypeName)}
	if col.TypeName != nil && len(col.TypeName.ArrayBounds) > 0 {
		c.Array = true
	}
	for _, cn := range col.Constraints {
		con := cn.GetConstraint()
		if con == nil {
			continue
		}
		if con.Contype == pgquery.ConstrType_CONSTR_NOTNULL || con.Contype == pgquery.ConstrType_CONSTR_PRIMARY {
			c.NotNull = true
		}
	}
	return c
}

func (c *Catalog) applyAlter(s *pgquery.AlterTableStmt) {
	if s.Relation == nil {
		return
	}
	t := c.byName[s.Relation.Relname]
	if t == nil {
		return
	}
	for _, cmd := range s.Cmds {
		ac := cmd.GetAlterTableCmd()
		if ac == nil {
			continue
		}
		switch ac.Subtype {
		case pgquery.AlterTableType_AT_AddColumn:
			if ac.Def != nil {
				if def := ac.Def.GetColumnDef(); def != nil {
					t.Columns = append(t.Columns, columnFromDef(def))
				}
			}
		case pgquery.AlterTableType_AT_DropColumn:
			t.dropColumn(ac.Name)
		}
	}
}

func (t *Table) dropColumn(name string) {
	t.Columns = slices.DeleteFunc(t.Columns, func(c Column) bool { return c.Name == name })
}

func (c *Catalog) applyDrop(s *pgquery.DropStmt) {
	if s.RemoveType != pgquery.ObjectType_OBJECT_TABLE {
		return
	}
	for _, obj := range s.Objects {
		name := dropName(obj)
		if name == "" {
			continue
		}
		if _, ok := c.byName[name]; !ok {
			continue
		}
		delete(c.byName, name)
		c.Tables = slices.DeleteFunc(c.Tables, func(t *Table) bool { return t.Name == name })
	}
}

func typeNameOf(tn *pgquery.TypeName) string {
	if tn == nil || len(tn.Names) == 0 {
		return ""
	}
	last := tn.Names[len(tn.Names)-1]
	if s := last.GetString_(); s != nil {
		return s.Sval
	}
	return ""
}

// dropName extracts a table name from a DropStmt object node (a List of String,
// or occasionally a bare String).
func dropName(node *pgquery.Node) string {
	if list := node.GetList(); list != nil {
		if n := len(list.Items); n > 0 {
			if s := list.Items[n-1].GetString_(); s != nil {
				return s.Sval
			}
		}
		return ""
	}
	if s := node.GetString_(); s != nil {
		return s.Sval
	}
	return ""
}
