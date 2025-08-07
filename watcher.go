package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	watcher      *fsnotify.Watcher
	config       *Config
	changeQueue  chan string
	logger       *log.Logger
}

func NewFileWatcher(config *Config, logger *log.Logger) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("创建文件监控器失败: %v", err)
	}

	fw := &FileWatcher{
		watcher:     watcher,
		config:      config,
		changeQueue: make(chan string, 1000),
		logger:      logger,
	}

	return fw, nil
}

func (fw *FileWatcher) Start() error {
	err := fw.addWatchRecursively(fw.config.LocalDirectory)
	if err != nil {
		return fmt.Errorf("添加监控目录失败: %v", err)
	}

	go fw.processEvents()
	fw.logger.Printf("文件监控启动，监控目录: %s", fw.config.LocalDirectory)
	return nil
}

func (fw *FileWatcher) Stop() {
	if fw.watcher != nil {
		fw.watcher.Close()
	}
	close(fw.changeQueue)
}

func (fw *FileWatcher) GetChangeQueue() <-chan string {
	return fw.changeQueue
}

func (fw *FileWatcher) processEvents() {
	debounceMap := make(map[string]*time.Timer)
	
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			if fw.shouldIgnoreFile(event.Name) {
				continue
			}

			if event.Op&fsnotify.Create == fsnotify.Create {
				info, err := os.Lstat(event.Name)
				if err == nil && info.IsDir() {
					fw.addWatchRecursively(event.Name)
				}
			}

			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				fw.debounceChange(debounceMap, event.Name)
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.logger.Printf("文件监控错误: %v", err)
		}
	}
}

func (fw *FileWatcher) debounceChange(debounceMap map[string]*time.Timer, filePath string) {
	if timer, exists := debounceMap[filePath]; exists {
		timer.Stop()
	}

	debounceMap[filePath] = time.AfterFunc(
		time.Duration(fw.config.Sync.SyncInterval)*time.Second,
		func() {
			select {
			case fw.changeQueue <- filePath:
			default:
				fw.logger.Printf("变更队列已满，跳过文件: %s", filePath)
			}
			delete(debounceMap, filePath)
		},
	)
}

func (fw *FileWatcher) addWatchRecursively(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fw.logger.Printf("访问路径失败 %s: %v", path, err)
			return nil
		}

		if info.IsDir() && !fw.shouldIgnoreFile(path) {
			err := fw.watcher.Add(path)
			if err != nil {
				fw.logger.Printf("添加监控路径失败 %s: %v", path, err)
				return nil
			}
			fw.logger.Printf("添加监控路径: %s", path)
		}

		return nil
	})
}

func (fw *FileWatcher) shouldIgnoreFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	
	for _, pattern := range fw.config.Sync.IgnorePatterns {
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