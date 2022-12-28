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

	"log"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/godror/godror"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

func main() {
	log.SetOutput(&lumberjack.Logger{
		Filename:   "./log/notifyChangeDB.log",
		MaxSize:    10,
		MaxBackups: 10,
		MaxAge:     28,
		Compress:   false,
	})

	cfg := config.NewConfig()
	err := cfg.LoadConfig("notifyChangeDB.conf")
	if err != nil {
		log.Printf("Failed to load config file: %v\n", err)
		return
	}
	if len(cfg.Cfg.Items) == 0 {
		log.Println("Nothing observe item")
		return
	}

	// 内部処理用DB
	dbfile := cfg.Cfg.PathDB
	db, err := sql.Open("sqlite3", dbfile)
	if err != nil {
		log.Printf("Failed to open sqlite3: %v\n", err)
		return
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS process (event text PRIMARY KEY, timestamp text, indicate_key text, indicate_value text)`)
	if err != nil {
		log.Printf("Failed to create table [sqlite3]: %v\n", err)
		return
	}
	// 内部処理用DBから、保存値を取得する
	for i := 0; i < len(cfg.Cfg.Items); i++ {
		id := cfg.Cfg.Items[i].ID
		// 取得できたら、内部値にする
		var last_value string
		err := db.QueryRow("SELECT indicate_value FROM process WHERE event = ?", id).Scan(&last_value)
		if err == nil {
			log.Printf("read value - %d %s: [%v]\n", i, id, last_value)
			cfg.Cfg.Items[i].IndicatorColunmValue = last_value
		}
	}

	driverName := cfg.Cfg.Driver
	connStr := cfg.Cfg.ConnectionString

	engine, err := xorm.NewEngine(driverName, connStr)
	if err != nil {
		log.Printf("Failed to open target DB: %v\n", err)
		return
	}

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)

	done := make(chan string)
	go func() {

		for {
			var sc = bufio.NewScanner(os.Stdin)
			if sc.Scan() {

				hasError := checkDatabase(cfg, engine, db)
				if hasError {
					return
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

func checkDatabase(cfg *config.Config, engine *xorm.Engine, db *sql.DB) bool {
	for i := 0; i < len(cfg.Cfg.Items); i++ {
		id := cfg.Cfg.Items[i].ID
		strFmtSQL := cfg.Cfg.Items[i].SqlTemplate
		colLastName := cfg.Cfg.Items[i].IndicatorColunmName
		colLastValue := cfg.Cfg.Items[i].IndicatorColunmValue

		strSQL := fmt.Sprintf(strFmtSQL, colLastValue)
		results, err := engine.QueryInterface(strSQL)
		if err != nil {
			log.Printf("Failed to query: %v\n", err)
			return true
		}
		for _, vs := range results {
			mapItem := make(map[string]interface{})
			for k, v := range vs {
				// Check NOT NULL
				if len(k) == 0 || v == nil {
					continue
				}

				mapItem[k] = v

				strValue := ""
				if vv, ok := v.(string); ok {
					strValue = vv
				}
				if colLastName == k && strValue > colLastValue {
					colLastValue = strValue
				}
			}
			b, err := json.Marshal(mapItem)
			if err == nil {
				strJSON := string(b)
				log.Println(strJSON)
				fmt.Println(strJSON)
			} else {
				log.Printf("json.Marshal Failed: %v\n", err)
			}
		}

		cfg.Cfg.Items[i].IndicatorColunmValue = colLastValue

		tmNow := time.Now()
		strNow := tmNow.Format("2006-01-02 15:04:05")
		_, err = db.Exec(`INSERT INTO process (event, timestamp, indicate_key, indicate_value) VALUES (?, ?, ?, ?) on conflict(event) do update set timestamp = ?, indicate_value = ?`, id, strNow, colLastName, colLastValue, strNow, colLastValue)
		if err != nil {
			log.Printf("Failed to exec: %v\n", err)
		}
	}
	return false
}
