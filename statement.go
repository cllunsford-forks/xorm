// Copyright 2013 The XORM Authors. All rights reserved.
// Use of this source code is governed by a BSD
// license that can be found in the LICENSE file.

// Package xorm provides is a simple and powerful ORM for Go. It makes your
// database operation simple.

package xorm

import (
	"fmt"
	"reflect"
	//"strconv"
	"strings"
	"time"
)

type Statement struct {
	RefTable     *Table
	Engine       *Engine
	Start        int
	LimitN       int
	WhereStr     string
	Params       []interface{}
	OrderStr     string
	JoinStr      string
	GroupByStr   string
	HavingStr    string
	ColumnStr    string
	columnMap    map[string]bool
	ConditionStr string
	AltTableName string
	RawSQL       string
	RawParams    []interface{}
	UseCascade   bool
	UseAutoJoin  bool
	StoreEngine  string
	Charset      string
	BeanArgs     []interface{}
}

func MakeArray(elem string, count int) []string {
	res := make([]string, count)
	for i := 0; i < count; i++ {
		res[i] = elem
	}
	return res
}

func (statement *Statement) Init() {
	statement.RefTable = nil
	statement.Start = 0
	statement.LimitN = 0
	statement.WhereStr = ""
	statement.Params = make([]interface{}, 0)
	statement.OrderStr = ""
	statement.UseCascade = true
	statement.JoinStr = ""
	statement.GroupByStr = ""
	statement.HavingStr = ""
	statement.ColumnStr = ""
	statement.columnMap = make(map[string]bool)
	statement.ConditionStr = ""
	statement.AltTableName = ""
	statement.RawSQL = ""
	statement.RawParams = make([]interface{}, 0)
	statement.BeanArgs = make([]interface{}, 0)
}

func (statement *Statement) Sql(querystring string, args ...interface{}) {
	statement.RawSQL = querystring
	statement.RawParams = args
}

func (statement *Statement) Where(querystring string, args ...interface{}) {
	statement.WhereStr = querystring
	statement.Params = args
}

func (statement *Statement) Table(tableName string) {
	statement.AltTableName = tableName
}

func BuildConditions(engine *Engine, table *Table, bean interface{}) ([]string, []interface{}) {
	colNames := make([]string, 0)
	var args = make([]interface{}, 0)
	for _, col := range table.Columns {
		fieldValue := col.ValueOf(bean)
		fieldType := reflect.TypeOf(fieldValue.Interface())
		val := fieldValue.Interface()
		switch fieldType.Kind() {
		case reflect.String:
			if fieldValue.String() == "" {
				continue
			}
		case reflect.Int, reflect.Int32, reflect.Int64:
			if fieldValue.Int() == 0 {
				continue
			}
		case reflect.Struct:
			if fieldType == reflect.TypeOf(time.Now()) {
				t := fieldValue.Interface().(time.Time)
				if t.IsZero() {
					continue
				}
			} else {
				engine.AutoMapType(fieldValue.Type())
			}
		default:
			continue
		}

		if table, ok := engine.Tables[fieldValue.Type()]; ok {
			pkField := reflect.Indirect(fieldValue).FieldByName(table.PKColumn().FieldName)
			if pkField.Int() != 0 {
				args = append(args, pkField.Interface())
			} else {
				continue
			}
		} else {
			args = append(args, val)
		}
		colNames = append(colNames, fmt.Sprintf("%v = ?", engine.Quote(col.Name)))
	}

	return colNames, args
}

func (statement *Statement) TableName() string {
	if statement.AltTableName != "" {
		return statement.AltTableName
	}

	if statement.RefTable != nil {
		return statement.RefTable.Name
	}
	return ""
}

func (statement *Statement) Id(id int64) {
	if statement.WhereStr == "" {
		statement.WhereStr = "(id)=?"
		statement.Params = []interface{}{id}
	} else {
		statement.WhereStr = statement.WhereStr + " and (id)=?"
		statement.Params = append(statement.Params, id)
	}
}

func (statement *Statement) In(column string, args ...interface{}) {
	inStr := fmt.Sprintf("%v IN (%v)", column, strings.Join(MakeArray("?", len(args)), ","))
	if statement.WhereStr == "" {
		statement.WhereStr = inStr
		statement.Params = args
	} else {
		statement.WhereStr = statement.WhereStr + " and " + inStr
		statement.Params = append(statement.Params, args...)
	}
}

func (statement *Statement) Cols(columns ...string) {
	statement.ColumnStr = strings.Join(columns, statement.Engine.Quote(", "))
	for _, column := range columns {
		statement.columnMap[column] = true
	}
}

func (statement *Statement) Limit(limit int, start ...int) {
	statement.LimitN = limit
	if len(start) > 0 {
		statement.Start = start[0]
	}
}

func (statement *Statement) OrderBy(order string) {
	statement.OrderStr = order
}

//The join_operator should be one of INNER, LEFT OUTER, CROSS etc - this will be prepended to JOIN
func (statement *Statement) Join(join_operator, tablename, condition string) {
	if statement.JoinStr != "" {
		statement.JoinStr = statement.JoinStr + fmt.Sprintf("%v JOIN %v ON %v", join_operator, tablename, condition)
	} else {
		statement.JoinStr = fmt.Sprintf("%v JOIN %v ON %v", join_operator, tablename, condition)
	}
}

func (statement *Statement) GroupBy(keys string) {
	statement.GroupByStr = keys
}

func (statement *Statement) Having(conditions string) {
	statement.HavingStr = fmt.Sprintf("HAVING %v", conditions)
}

func (statement *Statement) genColumnStr() string {
	table := statement.RefTable
	colNames := make([]string, 0)
	for _, col := range table.Columns {
		if col.MapType != ONLYTODB {
			colNames = append(colNames, statement.Engine.Quote(statement.TableName())+"."+statement.Engine.Quote(col.Name))
		}
	}
	return strings.Join(colNames, ", ")
}

func (statement *Statement) genCreateSQL() string {
	sql := "CREATE TABLE IF NOT EXISTS " + statement.Engine.Quote(statement.TableName()) + " ("
	for _, col := range statement.RefTable.Columns {
		sql += col.String(statement.Engine)
		sql = strings.TrimSpace(sql)
		sql += ", "
	}
	sql = sql[:len(sql)-2] + ")"
	if statement.Engine.Dialect.SupportEngine() && statement.StoreEngine != "" {
		sql += " ENGINE=" + statement.StoreEngine
	}
	if statement.Engine.Dialect.SupportCharset() && statement.Charset != "" {
		sql += " DEFAULT CHARSET " + statement.Charset
	}
	sql += ";"
	return sql
}

func (statement *Statement) genIndexSQL() []string {
	var sqls []string = make([]string, 0)
	for indexName, cols := range statement.RefTable.Indexes {
		sql := fmt.Sprintf("CREATE INDEX IDX_%v_%v ON %v (%v);", statement.TableName(), indexName,
			statement.TableName(), strings.Join(cols, ","))
		sqls = append(sqls, sql)
	}
	return sqls
}

func (statement *Statement) genUniqueSQL() []string {
	var sqls []string = make([]string, 0)
	for indexName, cols := range statement.RefTable.Uniques {
		sql := fmt.Sprintf("CREATE UNIQUE INDEX UQE_%v_%v ON %v (%v);", statement.TableName(), indexName,
			statement.TableName(), strings.Join(cols, ","))
		sqls = append(sqls, sql)
	}
	return sqls
}

func (statement *Statement) genDropSQL() string {
	sql := "DROP TABLE IF EXISTS " + statement.Engine.Quote(statement.TableName()) + ";"
	return sql
}

func (statement Statement) genGetSql(bean interface{}) (string, []interface{}) {
	table := statement.Engine.AutoMap(bean)
	statement.RefTable = table

	colNames, args := BuildConditions(statement.Engine, table, bean)
	statement.ConditionStr = strings.Join(colNames, " and ")
	statement.BeanArgs = args

	var columnStr string = statement.ColumnStr
	if columnStr == "" {
		columnStr = statement.genColumnStr()
	}

	return statement.genSelectSql(columnStr), append(statement.Params, statement.BeanArgs...)
}

func (statement Statement) genCountSql(bean interface{}) (string, []interface{}) {
	table := statement.Engine.AutoMap(bean)
	statement.RefTable = table

	colNames, args := BuildConditions(statement.Engine, table, bean)
	statement.ConditionStr = strings.Join(colNames, " and ")
	statement.BeanArgs = args
	return statement.genSelectSql(fmt.Sprintf("count(*) as %v", statement.Engine.Quote("total"))), append(statement.Params, statement.BeanArgs...)
}

func (statement Statement) genSelectSql(columnStr string) (a string) {
	if statement.GroupByStr != "" {
		columnStr = statement.Engine.Quote(strings.Replace(statement.GroupByStr, ",", statement.Engine.Quote(","), -1))
		statement.GroupByStr = columnStr
	}
	a = fmt.Sprintf("SELECT %v FROM %v", columnStr,
		statement.Engine.Quote(statement.TableName()))
	if statement.JoinStr != "" {
		a = fmt.Sprintf("%v %v", a, statement.JoinStr)
	}
	if statement.WhereStr != "" {
		a = fmt.Sprintf("%v WHERE %v", a, statement.WhereStr)
		if statement.ConditionStr != "" {
			a = fmt.Sprintf("%v and %v", a, statement.ConditionStr)
		}
	} else if statement.ConditionStr != "" {
		a = fmt.Sprintf("%v WHERE %v", a, statement.ConditionStr)
	}
	if statement.GroupByStr != "" {
		a = fmt.Sprintf("%v GROUP BY %v", a, statement.GroupByStr)
	}
	if statement.HavingStr != "" {
		a = fmt.Sprintf("%v %v", a, statement.HavingStr)
	}
	if statement.OrderStr != "" {
		a = fmt.Sprintf("%v ORDER BY %v", a, statement.OrderStr)
	}
	if statement.Start > 0 {
		a = fmt.Sprintf("%v LIMIT %v OFFSET %v", a, statement.LimitN, statement.Start)
	} else if statement.LimitN > 0 {
		a = fmt.Sprintf("%v LIMIT %v", a, statement.LimitN)
	}
	return
}
