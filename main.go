package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/godror/godror"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

func main() {
	dbType := "postgres"
	dic := map[string]struct {
		dbDriver     string
		conString    string
		sqlFormat    string
		colLastName  string
		colLastValue string
	}{
		"sqlite":    {dbDriver: "sqlite3", conString: "file:test.db?cache=shared&mode=rwc", sqlFormat: "SELECT * from win_cpu limit 100"},
		"postgres":  {dbDriver: "postgres", conString: "postgres://postgres:postgres@localhost/postgres?sslmode=disable", sqlFormat: `SELECT * from public.win_cpu where "time" > timestamp '%s' limit 100`, colLastName: "time", colLastValue: "1970-01-01 00:00:00"},
		"oracle":    {dbDriver: "godror", conString: `user="scott" password="tiger" connectString="dbhost:1521/orclpdb1"`, sqlFormat: "SELECT * from table_name WHERE ROWNUM <= 100"},
		"sqlserver": {dbDriver: "mssql", conString: "sqlserver://username:passwo%23rd@localhost/instance?database=databaseName&TrustServerCertificate=True", sqlFormat: "SELECT TOP 10 * from table_name;"},
	}

	driverName := dic[dbType].dbDriver
	connStr := dic[dbType].conString
	strFmtSQL := dic[dbType].sqlFormat
	colLastName := dic[dbType].colLastName
	colLastValue := dic[dbType].colLastValue

	engine, err := xorm.NewEngine(driverName, connStr)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)

	done := make(chan string)
	go func() {

		for {
			var sc = bufio.NewScanner(os.Stdin)
			if sc.Scan() {

				strSQL := fmt.Sprintf(strFmtSQL, colLastValue)
				results, err := engine.Query(strSQL)
				if err != nil {
					fmt.Println(err.Error())
					return
				}
				for _, vs := range results {
					mapItem := make(map[string]string)
					for k, v := range vs {
						// Check NOT NULL
						if len(k) == 0 || len(v) == 0 {
							continue
						}

						strValue := string(v)
						mapItem[k] = strValue

						if colLastName == k && strValue > colLastValue {
							colLastValue = strValue
						}
					}
					b, err := json.Marshal(mapItem)
					if err == nil {
						fmt.Println(string(b))
					}
				}
			} else {
				done <- "done"
			}
			if sc.Err() != nil {
				done <- "done"
				break
			}
		}
	}()

	select {
	case <-quit:
	case <-done:
	}
}
