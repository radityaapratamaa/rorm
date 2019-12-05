package rorm

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type (
	// RormEngine - Engine of Raw Query ORM Library
	RormEngine interface {
		SetTableOptions(tbCaseFormat, tbPrefix string)
		SetIsMultiRows(state bool)
		SetDB(db *sqlx.DB)
		GetDB() *sqlx.DB
		GetPreparedValues() []interface{}
		GetMultiPreparedValues() [][]interface{}

		PrepareData(ctx context.Context, command string, data interface{}) error
		ExecuteCUDQuery(ctx context.Context, preparedValue ...interface{}) (int64, error)
		Clear()
		GenerateSelectQuery()
		GenerateRawCUDQuery(command string, data interface{})

		GetLastQuery() string
		// GetResults() []map[string]string
		// GetSingleResult() map[string]string

		Select(col ...string) RormEngine
		SelectSum(col string, colAlias ...string) RormEngine
		SelectAverage(col string, colAlias ...string) RormEngine
		SelectMax(col string, colAlias ...string) RormEngine
		SelectMin(col string, colAlias ...string) RormEngine
		SelectCount(col string, colAlias ...string) RormEngine

		Where(col string, value interface{}, opt ...string) RormEngine
		WhereRaw(args string, value ...interface{}) RormEngine

		WhereIn(col string, listOfValues ...interface{}) RormEngine
		WhereNotIn(col string, listOfValues ...interface{}) RormEngine
		WhereLike(col, value string) RormEngine
		WhereBetween(col string, val1, val2 interface{})
		WhereNotBetween(col string, val1, val2 interface{})

		Or(col string, value interface{}, opt ...string) RormEngine
		OrIn(col string, listOfValues ...interface{}) RormEngine
		OrNotIn(col string, listOfValues ...interface{}) RormEngine
		OrLike(col, value string) RormEngine
		OrBetween(col string, val1, val2 interface{})
		OrNotBetween(col string, val1, val2 interface{})

		OrderBy(col, value string) RormEngine
		Asc(col string) RormEngine
		Desc(col string) RormEngine

		Limit(limit int, offset ...int) RormEngine
		From(tableName string) RormEngine

		SQLRaw(rawQuery string, values ...interface{}) RormEngine
		Get(pointerStruct interface{}) error

		Insert(data interface{}) error
		Update(data interface{}) error
		Delete(data interface{}) error
	}
	// RormTransaction - Transaction Structure
	RormTransaction struct {
	}
)
