package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	if runtime.GOOS == "windows" {
		if !checkSingleInstance() {
			showMessage("程序已在运行", "目录同步程序已经在运行中，请勿重复启动！", 0x30)
			return
		}
		hideConsoleWindow()
	}

	config, err := LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	logger := setupLogger(config)
	logger.Printf("=== 目录同步程序启动 ===")
	logger.Printf("程序版本: 1.0.0")
	logger.Printf("操作系统: %s", runtime.GOOS)

	syncService, err := NewDirectorySync(config, logger)
	if err != nil {
		logger.Fatalf("创建同步服务失败: %v", err)
	}

	err = syncService.Start()
	if err != nil {
		logger.Fatalf("启动同步服务失败: %v", err)
	}

	logger.Printf("同步服务已启动，按 Ctrl+C 停止程序")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			time.Sleep(10 * time.Minute)
			logger.Printf("同步服务运行中... (本地: %s -> 远程: %s)", 
				config.LocalDirectory, config.RemoteDirectory)
		}
	}()

	<-signalChan
	logger.Printf("接收到停止信号，正在关闭服务...")
	
	syncService.Stop()
	
	if runtime.GOOS == "windows" && mutex != 0 {
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		releaseMutex := kernel32.NewProc("ReleaseMutex")
		releaseMutex.Call(mutex)
		syscall.CloseHandle(syscall.Handle(mutex))
	}
	
	logger.Printf("=== 目录同步程序退出 ===")
}

func setupLogger(config *Config) *log.Logger {
	logFile, err := os.OpenFile(config.Log.File, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		log.Fatalf("打开日志文件失败: %v", err)
	}

	var writers []io.Writer
	writers = append(writers, logFile)

	if runtime.GOOS != "windows" || os.Getenv("DEBUG") == "1" {
		writers = append(writers, os.Stdout)
	}

	multiWriter := io.MultiWriter(writers...)
	
	logger := log.New(multiWriter, "", log.LstdFlags|log.Lshortfile)
	
	go rotateLogFile(config.Log.File, config.Log.MaxSizeMB, logger)
	
	return logger
}

func rotateLogFile(logFile string, maxSizeMB int, logger *log.Logger) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		info, err := os.Stat(logFile)
		if err != nil {
			continue
		}

		sizeMB := info.Size() / (1024 * 1024)
		if sizeMB > int64(maxSizeMB) {
			backupFile := fmt.Sprintf("%s.%s", logFile, time.Now().Format("20060102_150405"))
			os.Rename(logFile, backupFile)
			logger.Printf("日志文件已轮转: %s", backupFile)
		}
	}
}

var mutex uintptr

func checkSingleInstance() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	createMutexW := kernel32.NewProc("CreateMutexW")
	waitForSingleObject := kernel32.NewProc("WaitForSingleObject")
	
	mutexName, _ := syscall.UTF16PtrFromString("DirectorySync_SingleInstance_Mutex")
	
	// 创建或打开互斥体
	handle, _, _ := createMutexW.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	if handle == 0 {
		return false
	}
	
	// 尝试获取互斥体，0表示立即返回
	result, _, _ := waitForSingleObject.Call(handle, 0)
	if result != 0 { // 不是WAIT_OBJECT_0，说明已被其他实例占用
		syscall.CloseHandle(syscall.Handle(handle))
		return false
	}
	
	mutex = handle
	return true
}

func showMessage(title, message string, style uintptr) {
	if runtime.GOOS != "windows" {
		fmt.Printf("%s: %s\n", title, message)
		return
	}
	
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	
	messageBox.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), style)
}

func hideConsoleWindow() {
	if runtime.GOOS == "windows" {
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		user32 := syscall.NewLazyDLL("user32.dll")
		
		getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
		showWindow := user32.NewProc("ShowWindow")
		
		hwnd, _, _ := getConsoleWindow.Call()
		if hwnd != 0 {
			showWindow.Call(hwnd, 0)
		}
	}
}