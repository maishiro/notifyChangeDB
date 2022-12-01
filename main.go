package main

import (
	"fmt"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/godror/godror"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

func main() {
	dbType := "sqlite"
	dic := map[string]struct {
		dbDriver  string
		conString string
		selSQL    string
	}{
		"sqlite":    {dbDriver: "sqlite3", conString: "file:test.db?cache=shared&mode=rwc", selSQL: "SELECT * from win_cpu limit 100"},
		"postgres":  {dbDriver: "postgres", conString: "postgres://postgres:postgres@localhost/postgres?sslmode=disable", selSQL: "SELECT * from public.win_cpu limit 100"},
		"oracle":    {dbDriver: "godror", conString: `user="scott" password="tiger" connectString="dbhost:1521/orclpdb1"`, selSQL: "SELECT * from table_name WHERE ROWNUM <= 100"},
		"sqlserver": {dbDriver: "mssql", conString: "sqlserver://username:passwo%23rd@localhost/instance?database=databaseName&TrustServerCertificate=True", selSQL: "SELECT TOP 10 * from table_name;"},
	}

	driverName := dic[dbType].dbDriver
	connStr := dic[dbType].conString
	strSQL := dic[dbType].selSQL

	engine, err := xorm.NewEngine(driverName, connStr)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	results, err := engine.Query(strSQL)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	for _, vs := range results {
		for k, v := range vs {
			fmt.Println(k)
			fmt.Println(string(v))
		}
	}
}
