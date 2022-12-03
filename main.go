package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"notifyChangeDB/config"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/godror/godror"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

func main() {

	cfg := config.NewConfig()
	if cfg.LoadConfig("notifyChangeDB.conf") != nil {
		return
	}
	if len(cfg.Cfg.Items) == 0 {
		return
	}

	driverName := cfg.Cfg.Driver
	connStr := cfg.Cfg.ConnectionString

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

				for i := 0; i < len(cfg.Cfg.Items); i++ {
					strFmtSQL := cfg.Cfg.Items[i].SqlTemplate
					colLastName := cfg.Cfg.Items[i].IndicatorColunmName
					colLastValue := cfg.Cfg.Items[i].IndicatorColunmValue

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

					cfg.Cfg.Items[i].IndicatorColunmValue = colLastValue
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
