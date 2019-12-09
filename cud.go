package rorm

import (
	"context"
	"errors"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/radityaapratamaa/rorm/constants"
	"github.com/radityaapratamaa/rorm/lib"
)

func (re *Engine) Insert(data interface{}) (int64, error) {
	defer re.clearField()

	// ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	// defer cancel()
	ctx := context.Background()
	if err := re.PrepareData(ctx, "INSERT", data); err != nil {
		return 0, err
	}
	defer re.stmt.Close()
	return re.ExecuteCUDQuery(ctx, re.preparedValue...)
}

func (re *Engine) Update(data interface{}) (int64, error) {
	defer re.clearField()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := re.PrepareData(ctx, "UPDATE", data); err != nil {
		return 0, err
	}
	defer re.stmt.Close()
	dt := re.preparedValue
	return re.ExecuteCUDQuery(ctx, dt...)
}

func (re *Engine) Delete(data interface{}) (int64, error) {
	defer re.clearField()
	dVal := reflect.ValueOf(data)
	if err := lib.CheckDataKind(dVal, false); err != nil {
		return 0, err
	}
	if valid, err := re.isSoftDelete(data); valid && err == nil {
		return re.Cols("deleted_at").Update(data)
	} else if err != nil {
		return 0, err
	}
	command := "DELETE"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	re.GenerateRawCUDQuery(command, data)
	dt := re.preparedValue
	return re.ExecuteCUDQuery(ctx, dt...)
}

func (re *Engine) isSoftDelete(dt interface{}) (bool, error) {
	fieldElem := reflect.ValueOf(dt).Elem()
	for x := 0; x < fieldElem.NumField(); x++ {
		identifierTag := fieldElem.Type().Field(x).Tag.Get("rorm")
		if err := re.checkAndSetAutoColumn(identifierTag, fieldElem, "delete"); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (re *Engine) checkAndSetAutoColumn(identifierTag string, fieldElem reflect.Value, command string) error {
	var err error
	switch identifierTag {
	case "deleted":
		if err = re.setAutoUpdateCol(fieldElem, "DeletedAt"); (err == nil) && (command == "delete") {
			log.Println("set deleted at")
			return nil
		}

	case "created":
		if err = re.setAutoUpdateCol(fieldElem, "CreatedAt"); (err == nil) && (command == "insert") {
			log.Println("set created at")
			return nil
		}
	case "updated":
		if err = re.setAutoUpdateCol(fieldElem, "UpdatedAt"); (err == nil) && (command == "update") {
			log.Println("set updated at")
			return nil
		}
	}
	return err
}

func (re *Engine) setAutoUpdateCol(fieldElem reflect.Value, colName string) error {
	action := strings.ToLower(colName[:len(colName)-2])
	field := fieldElem.FieldByName(colName)
	if !field.IsValid() {
		return errors.New("Struct field name must be '" + colName + "' if it has a tag rorm '" + action + "'")
	}

	if field.CanSet() && strings.Contains(field.Type().String(), "time.Time") {
		currentTime := time.Now()
		currTimeReflectValue := reflect.ValueOf(currentTime)
		if field.Type().Kind() == reflect.Ptr {
			currTimeReflectValue = reflect.ValueOf(&currentTime)
		} else {
			return errors.New("Field '" + colName + "' must be time.Time data type")
		}
		field.Set(currTimeReflectValue)
	}
	return nil
}

func (re *Engine) PrepareData(ctx context.Context, command string, data interface{}) error {
	var err error
	dVal := reflect.ValueOf(data)
	isInsert := false
	if command == "INSERT" {
		isInsert = true
	}
	if err := lib.CheckDataKind(dVal, isInsert); err != nil {
		return err
	}
	if dVal.Elem().Kind() == reflect.Slice {
		re.PrepareMultiInsert(ctx, data)
		return nil
	}
	re.preparedData(command, data)
	re.GenerateRawCUDQuery(command, data)
	re.stmt, err = re.db.PreparexContext(ctx, re.rawQuery)
	if err != nil {
		return errors.New(constants.ErrPrepareStatement + err.Error())
	}
	return nil
}

func (re *Engine) Cols(col string, otherCols ...string) RormEngine {
	re.updatedCol = make(map[string]bool)
	re.updatedCol[col] = true
	if otherCols != nil {
		for _, col := range otherCols {
			re.updatedCol[col] = true
		}
	}
	return re
}
func (re *Engine) PrepareMultiInsert(ctx context.Context, data interface{}) error {
	sdValue := reflect.ValueOf(data).Elem()
	if sdValue.Len() == 0 {
		return errors.New("Data must be filled")
	}
	firstVal := sdValue.Index(0)
	if firstVal.Kind() == reflect.Ptr {
		firstVal = firstVal.Elem()
	}

	tableName := firstVal.Type().Name()
	if re.options.tbFormat == "snake" {
		tableName = lib.CamelToSnakeCase(tableName)
	} else {
		tableName = lib.SnakeToCamelCase(tableName)
	}
	tableName = re.syntaxQuote + tableName + re.syntaxQuote

	cols := "("
	val := cols
	for x := 0; x < firstVal.NumField(); x++ {
		colName, valid := re.getAndValidateTag(firstVal, x)
		if !valid {
			continue
		}
		fieldValue := firstVal.Field(x)
		cols += re.syntaxQuote + colName + re.syntaxQuote + ","
		val += "?,"

		re.preparedValue = append(re.preparedValue, fieldValue.Interface())
	}
	cols = cols[:len(cols)-1]
	val = val[:len(val)-1]
	cols += ")"
	val += ")"
	re.column = cols + " VALUES " + strings.Repeat(val+",", sdValue.Len()-1) + val
	re.rawQuery = "INSERT INTO " + tableName + " " + re.column
	re.rawQuery = re.db.Rebind(re.rawQuery)
	for x := 1; x < sdValue.Len(); x++ {
		tmpVal := sdValue.Index(x).Elem()
		for z := 0; z < tmpVal.NumField(); z++ {
			fieldType := tmpVal.Type().Field(z)
			fieldValue := tmpVal.Field(z)
			if _, valid := re.checkStructTag(fieldType.Tag, fieldValue); !valid {
				continue
			}
			re.preparedValue = append(re.preparedValue, tmpVal.Field(z).Interface())
		}
	}

	var err error
	re.stmt, err = re.db.PreparexContext(ctx, re.rawQuery)
	if err != nil {
		return errors.New(constants.ErrPrepareStatement + err.Error())
	}
	return nil
}

func (re *Engine) preparedData(command string, data interface{}) {
	sdValue := re.extractTableName(data)
	cols := "("
	values := "("
	if command == "UPDATE" {
		values = ""
	}
	var valid bool
	for x := 0; x < sdValue.NumField(); x++ {
		col := ""
		if col, valid = re.getAndValidateTag(sdValue, x); !valid {
			continue
		}
		if re.updatedCol != nil {
			if _, exist := re.updatedCol[col]; !exist {
				continue
			}
		}
		cols += re.syntaxQuote + col + re.syntaxQuote + ","
		if command == "INSERT" {
			values += "?,"
		} else {
			values += re.syntaxQuote + col + re.syntaxQuote + " = ?,"
		}
		re.preparedValue = append(re.preparedValue, sdValue.Field(x).Interface())
	}
	re.multiPreparedValue = append(re.multiPreparedValue, re.preparedValue)

	cols = cols[:len(cols)-1]
	values = values[:len(values)-1]
	cols += ")"
	if command == "INSERT" {
		values += ")"
		re.column = cols + " VALUES " + values
	} else if command == "UPDATE" {
		re.column = " SET " + values
	}
}

func (re *Engine) GenerateRawCUDQuery(command string, data interface{}) {
	re.rawQuery = command

	re.tableName = re.options.tbPrefix + re.tableName + re.options.tbPostfix
	// Adjustment Table Name to Case Format (if available)
	switch re.options.tbFormat {
	case "camel":
		re.tableName = lib.SnakeToCamelCase(re.tableName)
	case "snake":
		re.tableName = lib.CamelToSnakeCase(re.tableName)
	}
	if command == "INSERT" {
		re.rawQuery += " INTO "
	} else if command == "DELETE" {
		re.rawQuery += " FROM "
	}
	re.rawQuery += " " + re.tableName + " " + re.column

	// re.rawQuery = re.adjustPreparedParam(re.rawQuery)
	if re.condition != "" {
		re.convertToPreparedCondition()
		re.rawQuery += " WHERE "
		re.rawQuery += re.condition
	}
	re.rawQuery = re.db.Rebind(re.rawQuery)
}

func (re *Engine) ExecuteCUDQuery(ctx context.Context, preparedValue ...interface{}) (int64, error) {
	var affectedRows int64
	// for _, pv := range re.multiPreparedValue {
	if _, err := re.stmt.ExecContext(ctx, preparedValue...); err != nil {
		log.Println(err)
		return int64(0), err
	}
	affectedRows++
	return affectedRows, nil
}
