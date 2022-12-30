package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"strconv"
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
			cfg.Cfg.Items[i].IndicatorColumnValue = last_value
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
				// DB更新チェック
				if checkDatabase(cfg, engine, db) {
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

// DB更新チェック
func checkDatabase(cfg *config.Config, engine *xorm.Engine, db *sql.DB) bool {
	for i := 0; i < len(cfg.Cfg.Items); i++ {
		id := cfg.Cfg.Items[i].ID
		strFmtSQL := cfg.Cfg.Items[i].SqlTemplate
		colLastName := cfg.Cfg.Items[i].IndicatorColumnName
		colLastValue := cfg.Cfg.Items[i].IndicatorColumnValue
		tags := cfg.Cfg.Items[i].Tags
		mapTags := make(map[string]string)
		for _, v := range tags {
			mapTags[v] = ""
		}
		excludes := cfg.Cfg.Items[i].ExcludeColumns
		mapExcludes := make(map[string]string)
		for _, v := range excludes {
			mapExcludes[v] = ""
		}
		colTypes := cfg.Cfg.Items[i].ColumnTypes

		strTable := fmt.Sprintf("last_%s", id)
		// 差分保存用テーブル作成 (SQLite3)
		prepareDiffTable(db, strTable)
		// 差分データ読み込み (SQLite3)
		mapLast := loadDiffData(db, strTable)
		if mapLast == nil {
			return true
		}

		// DB更新データ取得
		strSQL := fmt.Sprintf(strFmtSQL, colLastValue)
		results, err := engine.QueryInterface(strSQL)
		if err != nil {
			log.Printf("Failed to query: %v\n", err)
			return true
		}

		for _, vs := range results {
			tags := make(map[string]interface{})
			field := make(map[string]interface{})
			mapItem := make(map[string]interface{})
			for k, v := range vs {
				// Check NOT NULL
				if len(k) == 0 || v == nil {
					continue
				}

				strValue := ""
				if vv, ok := v.(string); ok {
					strValue = vv
				} else {
					switch t := v.(type) {
					case []uint8:
						strValue = string(t)
					case int32:
						strValue = fmt.Sprint(int(t))
					case float64:
						strValue = fmt.Sprint(float64(t))
					default:
						log.Printf("default type: %v\n", t)
						strValue = fmt.Sprintf("%v", t)
					}
				}

				//
				if k == colLastName {
					//
				} else if _, ok := mapTags[k]; ok {
					tags[k] = strValue
				} else if _, ok := mapExcludes[k]; !ok {
					field[k] = strValue
				}

				parseValue(strValue, k, v, colTypes, mapItem)

				if colLastName == k && strValue > colLastValue {
					colLastValue = strValue
				}
			}

			// 差分用データ保存 (SQLite3)
			strJsonTags, strJsonFields := saveDiffData(db, strTable, tags, field)
			// 差分データ検証
			if checkDiff(mapLast, strJsonTags, strJsonFields) {
				// 差分出力
				outputDiff(os.Stdout, mapItem)
			}
		}

		cfg.Cfg.Items[i].IndicatorColumnValue = colLastValue

		strNow := time.Now().Format("2006-01-02 15:04:05")
		_, err = db.Exec(`INSERT INTO process (event, timestamp, indicate_key, indicate_value) VALUES (?, ?, ?, ?) on conflict(event) do update set timestamp = ?, indicate_value = ?`, id, strNow, colLastName, colLastValue, strNow, colLastValue)
		if err != nil {
			log.Printf("Failed to exec: %v\n", err)
		}
	}
	return false
}

func parseValue(strValue string, k string, v interface{}, colTypes map[string]string, mapItem map[string]interface{}) {
	if colType, ok := colTypes[k]; ok {

		switch colType {
		case "int":
			if vi, ok := v.(int); ok {
				mapItem[k] = vi
			} else {
				log.Printf("Failed to parse(%s): [%v]:[%v](%v)\n", colType, k, v, strValue)
				mapItem[k] = strValue
			}
		case "int32":
			i32, err := strconv.ParseInt(strValue, 10, 32)
			if err == nil {
				mapItem[k] = i32
			} else {
				log.Printf("Failed to parse(%s): [%v]:[%v](%v)\n", colType, k, v, strValue)
				mapItem[k] = strValue
			}
		case "int64":
			i64, err := strconv.ParseInt(strValue, 10, 64)
			if err == nil {
				mapItem[k] = i64
			} else {
				log.Printf("Failed to parse(%s): [%v]:[%v](%v)\n", colType, k, v, strValue)
				mapItem[k] = strValue
			}
		case "uint":
			if vi, ok := v.(uint); ok {
				mapItem[k] = vi
			} else {
				log.Printf("Failed to parse(%s): [%v]:[%v](%v)\n", colType, k, v, strValue)
				mapItem[k] = strValue
			}
		case "uint32":
			ui, err := strconv.ParseUint(strValue, 10, 32)
			if err == nil {
				mapItem[k] = ui
			} else {
				log.Printf("Failed to parse(%s): [%v]:[%v](%v)\n", colType, k, v, strValue)
				mapItem[k] = strValue
			}
		case "uint64":
			ui, err := strconv.ParseUint(strValue, 10, 64)
			if err == nil {
				mapItem[k] = ui
			} else {
				log.Printf("Failed to parse(%s): [%v]:[%v](%v)\n", colType, k, v, strValue)
				mapItem[k] = strValue
			}
		case "float32":
			f32, err := strconv.ParseFloat(strValue, 32)
			if err == nil {
				mapItem[k] = f32
			} else {
				log.Printf("Failed to parse(%s): [%v]:[%v](%v)\n", colType, k, v, strValue)
				mapItem[k] = strValue
			}
		case "float64":
			f64, err := strconv.ParseFloat(strValue, 64)
			if err == nil {
				mapItem[k] = f64
			} else {
				log.Printf("Failed to parse(%s): [%v]:[%v](%v)\n", colType, k, v, strValue)
				mapItem[k] = strValue
			}
		}
	} else {
		mapItem[k] = strValue
	}
}

// 差分保存用テーブル作成 (SQLite3)
func prepareDiffTable(db *sql.DB, strTable string) {
	sqlDDL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (tags text PRIMARY KEY, fields text)`, strTable)
	_, err := db.Exec(sqlDDL)
	if err != nil {
		log.Printf("Failed to create table [sqlite3]: %v\n", err)
	}
}

// 差分データ読み込み (SQLite3)
func loadDiffData(db *sql.DB, strTable string) map[string]interface{} {
	sqlLast := fmt.Sprintf(`SELECT tags, fields FROM %s`, strTable)
	lastItems, err := db.Query(sqlLast)
	if err != nil {
		log.Printf("Failed to query: %v\n", err)
		return nil
	}
	mapLast := make(map[string]interface{})
	for lastItems.Next() {
		var strT string
		var strF string
		_ = lastItems.Scan(&strT, &strF)
		mapLast[strT] = strF
	}
	return mapLast
}

// 差分用データ保存 (SQLite3)
func saveDiffData(db *sql.DB, strTable string, tags map[string]interface{}, field map[string]interface{}) (string, string) {
	strJsonTags := ""
	strJsonFields := ""
	b1, err1 := json.Marshal(tags)
	b2, err2 := json.Marshal(field)
	if 0 < len(tags) && err1 == nil && err2 == nil {
		strJsonTags = string(b1)
		strJsonFields = string(b2)
		sqlLeft := fmt.Sprintf(`INSERT OR REPLACE INTO %s (tags, fields) VALUES (?, ?)`, strTable)
		_, err := db.Exec(sqlLeft, strJsonTags, strJsonFields)
		if err != nil {
			log.Printf("Failed to exec: %v\n", err)
		}
	}
	return strJsonTags, strJsonFields
}

// 差分データ検証
func checkDiff(mapLast map[string]interface{}, strJsonTags string, strJsonFields string) bool {
	bSave := true
	for k, v := range mapLast {
		if IsEqualJSON(k, strJsonTags) {
			if IsEqualJSON(JsonString(v), strJsonFields) {
				bSave = false
			}
			break
		}
	}
	return bSave
}

// 差分出力
func outputDiff(w io.Writer, mapItem map[string]interface{}) {
	b, err := json.Marshal(mapItem)
	if err == nil {
		strJSON := string(b)
		log.Println(strJSON)
		fmt.Fprintln(w, strJSON)
	} else {
		log.Printf("json.Marshal Failed: %v\n", err)
	}
}

func JsonString(v interface{}) string {
	strJSON := ""
	b, err := json.Marshal(v)
	if err == nil {
		strJSON = string(b)
	}
	return strJSON
}

func DeepEqualJSON(j1, j2 string) (bool, error) {
	var err error

	var d1 interface{}
	err = json.Unmarshal([]byte(j1), &d1)
	if err != nil {
		return false, err
	}

	var d2 interface{}
	err = json.Unmarshal([]byte(j2), &d2)
	if err != nil {
		return false, err
	}

	if reflect.DeepEqual(d1, d2) {
		return true, nil
	} else {
		return false, nil
	}
}

func IsEqualJSON(a, b string) bool {
	bEqual, _ := DeepEqualJSON(a, b)
	return bEqual
}
