package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DirectorySync struct {
	config     *Config
	sftpClient *SFTPClient
	watcher    *FileWatcher
	logger     *log.Logger
	running    bool
}

func NewDirectorySync(config *Config, logger *log.Logger) (*DirectorySync, error) {
	sftpClient, err := NewSFTPClient(config)
	if err != nil {
		return nil, fmt.Errorf("SFTP客户端创建失败: %v", err)
	}

	watcher, err := NewFileWatcher(config, logger)
	if err != nil {
		sftpClient.Close()
		return nil, fmt.Errorf("文件监控器创建失败: %v", err)
	}

	return &DirectorySync{
		config:     config,
		sftpClient: sftpClient,
		watcher:    watcher,
		logger:     logger,
		running:    false,
	}, nil
}

func (ds *DirectorySync) Start() error {
	if ds.running {
		return fmt.Errorf("同步服务已在运行")
	}

	err := ds.watcher.Start()
	if err != nil {
		return fmt.Errorf("文件监控启动失败: %v", err)
	}

	ds.running = true
	ds.logger.Printf("目录同步服务启动")
	ds.logger.Printf("本地目录: %s", ds.config.LocalDirectory)
	ds.logger.Printf("远程目录: %s", ds.config.RemoteDirectory)

	err = ds.performInitialSync()
	if err != nil {
		ds.logger.Printf("初始同步失败: %v", err)
	}

	go ds.processChanges()
	go ds.keepAlive()

	return nil
}

func (ds *DirectorySync) Stop() {
	if !ds.running {
		return
	}

	ds.running = false
	ds.watcher.Stop()
	ds.sftpClient.Close()
	ds.logger.Printf("目录同步服务停止")
}

func (ds *DirectorySync) performInitialSync() error {
	ds.logger.Printf("开始初始同步...")
	
	return filepath.Walk(ds.config.LocalDirectory, func(localPath string, info os.FileInfo, err error) error {
		if err != nil {
			ds.logger.Printf("访问文件失败 %s: %v", localPath, err)
			return nil
		}

		if ds.shouldIgnoreFile(localPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() {
			remotePath := ds.getRemotePath(localPath)
			err := ds.uploadFile(localPath, remotePath)
			if err != nil {
				ds.logger.Printf("初始同步文件失败 %s: %v", localPath, err)
			}
		}

		return nil
	})
}

func (ds *DirectorySync) processChanges() {
	for filePath := range ds.watcher.GetChangeQueue() {
		if !ds.running {
			break
		}

		ds.handleFileChange(filePath)
	}
}

func (ds *DirectorySync) handleFileChange(localPath string) {
	if ds.shouldIgnoreFile(localPath) {
		return
	}

	if !ds.ensureConnection() {
		return
	}

	remotePath := ds.getRemotePath(localPath)

	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			ds.handleFileDelete(remotePath)
		} else {
			ds.logger.Printf("获取文件信息失败 %s: %v", localPath, err)
		}
		return
	}

	if info.IsDir() {
		return
	}

	err = ds.uploadFile(localPath, remotePath)
	if err != nil {
		ds.logger.Printf("文件上传失败 %s: %v", localPath, err)
	} else {
		ds.logger.Printf("文件已同步: %s -> %s", localPath, remotePath)
	}
}

func (ds *DirectorySync) handleFileDelete(remotePath string) {
	if ds.config.Sync.DeleteRemote {
		err := ds.sftpClient.RemoveFile(remotePath)
		if err != nil {
			ds.logger.Printf("删除远程文件失败 %s: %v", remotePath, err)
		} else {
			ds.logger.Printf("远程文件已删除: %s", remotePath)
		}
	}
}

func (ds *DirectorySync) uploadFile(localPath, remotePath string) error {
	maxRetries := 3
	var err error

	for i := 0; i < maxRetries; i++ {
		if !ds.ensureConnection() {
			return fmt.Errorf("SFTP连接失败")
		}

		err = ds.sftpClient.UploadFile(localPath, remotePath)
		if err == nil {
			return nil
		}

		ds.logger.Printf("上传失败 (尝试 %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Second * time.Duration(i+1))
	}

	return err
}

func (ds *DirectorySync) ensureConnection() bool {
	if ds.sftpClient.IsConnected() {
		return true
	}

	ds.logger.Printf("SFTP连接断开，尝试重连...")
	err := ds.sftpClient.Reconnect()
	if err != nil {
		ds.logger.Printf("SFTP重连失败: %v", err)
		return false
	}

	ds.logger.Printf("SFTP重连成功")
	return true
}

func (ds *DirectorySync) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !ds.running {
				return
			}
			if !ds.sftpClient.IsConnected() {
				ds.logger.Printf("检测到连接断开")
				ds.ensureConnection()
			}
		}
	}
}

func (ds *DirectorySync) getRemotePath(localPath string) string {
	relPath, err := filepath.Rel(ds.config.LocalDirectory, localPath)
	if err != nil {
		ds.logger.Printf("获取相对路径失败: %v", err)
		return ""
	}

	remotePath := filepath.Join(ds.config.RemoteDirectory, relPath)
	return strings.ReplaceAll(remotePath, "\\", "/")
}

func (ds *DirectorySync) shouldIgnoreFile(filePath string) bool {
	fileName := filepath.Base(filePath)

	for _, pattern := range ds.config.Sync.IgnorePatterns {
		matched, err := filepath.Match(pattern, fileName)
		if err == nil && matched {
			return true
		}

		if strings.Contains(filePath, pattern) {
			return true
		}
	}

	return false
}