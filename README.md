# notifyChangeDB
 
telegrafツールでInput Plugin を execd としたときに、実行ファイルに使用する。  
DBに追加・変更されたレコードを取得してJSON出力する。  

## 使い方

1. DBに追加・変更されたレコードを取得するための設定をファイル（notifyChangeDB.conf）へ記述  
- DB接続設定
- ID
- SQL

2. telegrafの設定ファイルでInput Pluginをexecdとし、実行するプログラムとして設定  
- signal設定
- JSONのパース設定

 configファイル設定例
```
[[inputs.execd]]
  command = ["C:\\work\\notifyChangeDB.exe"]
  signal = "STDIN"

  data_format = "xpath_json"

  [[inputs.execd.xpath]]
    metric_name = "/objectname"
    timestamp = "/time"
    timestamp_format = "2006-01-02 15:04:05"
    field_selection = "/*"
```

## Requirement
* Windows
* Go


