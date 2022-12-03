package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

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

	// 内部処理用DB
	dbfile := cfg.Cfg.PathDB
	db, err := sql.Open("sqlite3", dbfile)
	if err != nil {
		return
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS process (event text PRIMARY KEY, timestamp text, indicate_key text, indicate_value text)`)
	if err != nil {
		return
	}
	// 内部処理用DBから、保存値を取得する
	for i := 0; i < len(cfg.Cfg.Items); i++ {
		id := cfg.Cfg.Items[i].ID
		// 取得できたら、内部値にする
		var last_value string
		err := db.QueryRow("SELECT indicate_value FROM process WHERE event = ?", id).Scan(&last_value)
		if err == nil {
			cfg.Cfg.Items[i].IndicatorColunmValue = last_value
		}
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
					id := cfg.Cfg.Items[i].ID
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

					tmNow := time.Now()
					strNow := tmNow.Format("2006-01-02 15:04:05")
					_, err = db.Exec(`INSERT INTO process (event, timestamp, indicate_key, indicate_value) VALUES (?, ?, ?, ?) on conflict(event) do update set timestamp = ?, indicate_value = ?`, id, strNow, colLastName, colLastValue, strNow, colLastValue)
					if err != nil {
						fmt.Println(err.Error())
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
