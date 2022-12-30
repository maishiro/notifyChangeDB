package main

import (
	"bytes"
	"testing"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/godror/godror"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func Test_outputDiff(t *testing.T) {
	type args struct {
		mapItem map[string]interface{}
	}
	tests := []struct {
		name   string
		args   args
		expect string
	}{
		{
			name:   "test1",
			args:   args{mapItem: map[string]interface{}{"test": 1}},
			expect: `{"test":1}`,
		},
		{
			name:   "test2",
			args:   args{mapItem: map[string]interface{}{"b": "text2", "a": "text1"}},
			expect: `{"a":"text1","b":"text2"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			outputDiff(&output, tt.args.mapItem)

			if !IsEqualJSON(output.String(), tt.expect) {
				t.Errorf("value [%s] expected [%v]", output.String(), tt.args.mapItem)
			}
		})
	}
}

func Test_parseValue(t *testing.T) {
	type args struct {
		strValue string
		k        string
		v        interface{}
		colTypes map[string]string
		mapItem  map[string]interface{}
	}
	tests := []struct {
		name     string
		args     args
		expected string
	}{
		{
			name: "test0",
			args: args{
				strValue: "text1",
				k:        "value0",
				v:        interface{}("text1"),
				colTypes: map[string]string{},
				mapItem:  map[string]interface{}{},
			},
			expected: `{"value0":"text1"}`,
		},
		{
			name: "test1",
			args: args{
				strValue: "1",
				k:        "value1",
				v:        interface{}(1),
				colTypes: map[string]string{"value1": "int32"},
				mapItem:  map[string]interface{}{},
			},
			expected: `{"value1":1}`,
		},
		{
			name: "test2",
			args: args{
				strValue: "1.2",
				k:        "value2",
				v:        interface{}([]uint8("1.2")),
				colTypes: map[string]string{"value2": "float64"},
				mapItem:  map[string]interface{}{},
			},
			expected: `{"value2":1.2}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseValue(tt.args.strValue, tt.args.k, tt.args.v, tt.args.colTypes, tt.args.mapItem)
			j1 := tt.expected
			j2 := JsonString(tt.args.mapItem)
			if !IsEqualJSON(j1, j2) {
				t.Errorf("value [%v] expected [%v]", j1, j2)
			}
		})
	}
}
