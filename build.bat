@REM Build SQLite for Windows
@REM https://github.com/mattn/go-sqlite3#windows

@cd /d %~dp0

@rem set PATH=C:\TDM-GCC-64\bin;%PATH%
call C:\TDM-GCC-64\mingwvars.bat

go build

@pause
