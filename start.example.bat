@echo off
cd "your project directory"

start /min cmd /k "go run main.go"

timeout /t 5 /nobreak >nul 2>&1

rem Start Google Chrome in full screen mode (default windows chrome directory)
start "" "C:\Program Files\Google\Chrome\Application\chrome.exe" --start-fullscreen http://127.0.0.1:8080/
