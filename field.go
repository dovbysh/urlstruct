package urlstruct

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/vmihailenco/tagparser"

	"github.com/go-pg/urlstruct/internal"
	"github.com/go-pg/zerochecker"
)

type OpCode int

const (
	OpEq OpCode = iota + 1
	OpNotEq
	OpLT
	OpLTE
	OpGT
	OpGTE
	OpIEq
	OpMatch
)

type Field struct {
	Type   reflect.Type
	Name   string
	Index  []int
	Column string
	Op     OpCode

	noDecode bool
	required bool
	noWhere  bool

	scanValue   scannerFunc
	isZeroValue zerochecker.Func
}

func newField(meta *StructInfo, sf reflect.StructField, baseIndex []int) *Field {
	tag := tagparser.Parse(sf.Tag.Get("urlstruct"))
	if tag.Name == "-" {
		return nil
	}

	if _, ok := tag.Options["unknown"]; ok {
		meta.unknownFieldsIndex = append(baseIndex, sf.Index...)
		return nil
	}

	f := &Field{
		Type:  sf.Type,
		Name:  sf.Name,
		Index: append(baseIndex, sf.Index...),
	}

	if tag.Name != "" {
		f.Name = tag.Name
	}

	_, f.required = tag.Options["required"]
	_, f.noDecode = tag.Options["nodecode"]
	_, f.noWhere = tag.Options["nowhere"]
	if f.required && f.noWhere {
		err := fmt.Errorf("urlstruct: required and nowhere tags can't be set together")
		panic(err)
	}

	name := internal.Underscore(f.Name)
	const sep = "_"
	f.Column, f.Op = splitColumnOperator(name, sep)

	if f.Type.Kind() == reflect.Slice {
		f.scanValue = sliceScanner(sf.Type)
	} else {
		f.scanValue = scanner(sf.Type)
	}
	f.isZeroValue = zerochecker.Checker(sf.Type)

	if f.scanValue == nil || f.isZeroValue == nil {
		return nil
	}

	return f
}

func (f *Field) Value(strct reflect.Value) reflect.Value {
	return strct.FieldByIndex(f.Index)
}

func (f *Field) Omit(value reflect.Value) bool {
	return !f.required && f.noWhere || f.isZeroValue(value)
}

func splitColumnOperator(s, sep string) (string, OpCode) {
	ind := strings.LastIndex(s, sep)
	if ind == -1 {
		return s, OpEq
	}

	col := s[:ind]
	op := s[ind+len(sep):]

	switch op {
	case "eq", "":
		return col, OpEq
	case "neq", "exclude":
		return col, OpNotEq
	case "gt":
		return col, OpGT
	case "gte":
		return col, OpGTE
	case "lt":
		return col, OpLT
	case "lte":
		return col, OpLTE
	case "ieq":
		return col, OpIEq
	case "match":
		return col, OpMatch
	default:
		return s, OpEq
	}
}
