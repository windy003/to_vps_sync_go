@echo off
echo 开始编译目录同步程序...

:: 设置环境变量
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

:: 创建输出目录
if not exist "dist" mkdir dist

:: 编译程序 (隐藏控制台窗口)
echo 编译隐藏控制台版本...
go build -ldflags="-s -w -H windowsgui" -o dist/directory-sync.exe

if %errorlevel% neq 0 (
    echo 编译失败！
    pause
    exit /b 1
)

:: 编译调试版本 (显示控制台)
echo 编译调试版本...
go build -ldflags="-s -w" -o dist/directory-sync-debug.exe

if %errorlevel% neq 0 (
    echo 调试版本编译失败！
    pause
    exit /b 1
)

:: 复制配置文件
copy config.yaml dist\config.yaml >nul

echo.
echo ===============================================
echo 编译完成！输出文件：
echo   dist/directory-sync.exe       - 后台运行版本
echo   dist/directory-sync-debug.exe - 调试版本
echo   dist/config.yaml              - 配置文件
echo ===============================================
echo.
echo 使用说明：
echo 1. 修改 dist/config.yaml 中的配置
echo 2. 运行 directory-sync.exe（后台无窗口运行）
echo 3. 或运行 directory-sync-debug.exe（显示日志窗口）
echo.
pause