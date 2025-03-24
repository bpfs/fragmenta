package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ConfigChangeEntry 配置变更记录项
type ConfigChangeEntry struct {
	// 变更时间
	Timestamp time.Time `json:"timestamp"`

	// 变更源(文件路径或来源说明)
	Source string `json:"source"`

	// 变更描述
	Description string `json:"description"`

	// 旧配置(可选，完整配置可能较大)
	OldConfig *Config `json:"oldConfig,omitempty"`

	// 新配置(可选，完整配置可能较大)
	NewConfig *Config `json:"newConfig,omitempty"`

	// 变更的配置键值对
	Changes map[string]interface{} `json:"changes,omitempty"`
}

// ConfigChangeLogger 配置变更日志记录器
type ConfigChangeLogger struct {
	// 日志目录
	logDir string

	// 是否记录完整配置
	logFullConfig bool

	// 日志文件句柄
	logFile *os.File

	// 变更计数
	changeCount int

	// 互斥锁
	mu sync.Mutex

	// 最大日志文件大小（字节）
	maxLogSize int64

	// 最大历史文件数
	maxHistoryFiles int
}

// NewConfigChangeLogger 创建配置变更日志记录器
func NewConfigChangeLogger(logDir string) (*ConfigChangeLogger, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	logger := &ConfigChangeLogger{
		logDir:          logDir,
		logFullConfig:   false,
		changeCount:     0,
		maxLogSize:      10 * 1024 * 1024, // 默认10MB
		maxHistoryFiles: 5,                // 默认保留5个历史文件
	}

	// 打开当前日志文件
	if err := logger.openLogFile(); err != nil {
		return nil, err
	}

	return logger, nil
}

// SetLogFullConfig 设置是否记录完整配置
func (l *ConfigChangeLogger) SetLogFullConfig(logFullConfig bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logFullConfig = logFullConfig
}

// SetMaxLogSize 设置最大日志文件大小
func (l *ConfigChangeLogger) SetMaxLogSize(maxSizeMB int) {
	if maxSizeMB <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxLogSize = int64(maxSizeMB) * 1024 * 1024
}

// SetMaxHistoryFiles 设置最大历史文件数
func (l *ConfigChangeLogger) SetMaxHistoryFiles(maxFiles int) {
	if maxFiles <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxHistoryFiles = maxFiles
}

// LogConfigChange 记录配置变更
func (l *ConfigChangeLogger) LogConfigChange(oldConfig, newConfig *Config, source, description string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查文件大小，必要时执行轮转
	if l.logFile != nil {
		info, err := l.logFile.Stat()
		if err == nil && info.Size() >= l.maxLogSize {
			if err := l.rotateLogFile(); err != nil {
				return fmt.Errorf("failed to rotate log file: %v", err)
			}
		}
	}

	// 创建变更记录
	entry := ConfigChangeEntry{
		Timestamp:   time.Now(),
		Source:      source,
		Description: description,
		Changes:     make(map[string]interface{}),
	}

	// 如果需要记录完整配置
	if l.logFullConfig {
		entry.OldConfig = oldConfig
		entry.NewConfig = newConfig
	} else {
		// 否则只记录变更的部分
		entry.Changes = l.diffConfigs(oldConfig, newConfig)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config change entry: %v", err)
	}

	// 写入日志文件
	if l.logFile != nil {
		if _, err := l.logFile.WriteString(string(data) + "\n"); err != nil {
			return fmt.Errorf("failed to write to log file: %v", err)
		}
		if err := l.logFile.Sync(); err != nil {
			return fmt.Errorf("failed to sync log file: %v", err)
		}
	}

	l.changeCount++
	return nil
}

// openLogFile 打开日志文件
func (l *ConfigChangeLogger) openLogFile() error {
	logPath := filepath.Join(l.logDir, "config_changes.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	l.logFile = file
	return nil
}

// rotateLogFile 轮转日志文件
func (l *ConfigChangeLogger) rotateLogFile() error {
	// 关闭当前文件
	if l.logFile != nil {
		if err := l.logFile.Close(); err != nil {
			return err
		}
		l.logFile = nil
	}

	// 当前日志文件路径
	currentLogPath := filepath.Join(l.logDir, "config_changes.log")

	// 生成带时间戳的新文件名
	timestamp := time.Now().Format("20060102-150405")
	newLogPath := filepath.Join(l.logDir, fmt.Sprintf("config_changes-%s.log", timestamp))

	// 重命名当前日志文件
	if err := os.Rename(currentLogPath, newLogPath); err != nil {
		return err
	}

	// 清理旧的日志文件
	if err := l.cleanOldLogs(); err != nil {
		return err
	}

	// 重新打开一个新的日志文件
	return l.openLogFile()
}

// cleanOldLogs 清理老的日志文件
func (l *ConfigChangeLogger) cleanOldLogs() error {
	// 读取日志目录中的所有文件
	files, err := filepath.Glob(filepath.Join(l.logDir, "config_changes-*.log"))
	if err != nil {
		return err
	}

	// 如果文件数超过最大限制，删除最老的文件
	if len(files) > l.maxHistoryFiles {
		// 按修改时间排序
		type fileInfo struct {
			path    string
			modTime time.Time
		}
		fileInfos := make([]fileInfo, 0, len(files))

		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}
			fileInfos = append(fileInfos, fileInfo{path: file, modTime: info.ModTime()})
		}

		// 按修改时间排序
		for i := 0; i < len(fileInfos); i++ {
			for j := i + 1; j < len(fileInfos); j++ {
				if fileInfos[i].modTime.After(fileInfos[j].modTime) {
					fileInfos[i], fileInfos[j] = fileInfos[j], fileInfos[i]
				}
			}
		}

		// 删除最老的文件
		for i := 0; i < len(fileInfos)-l.maxHistoryFiles; i++ {
			if err := os.Remove(fileInfos[i].path); err != nil {
				return err
			}
		}
	}

	return nil
}

// Close 关闭日志记录器
func (l *ConfigChangeLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil {
		if err := l.logFile.Close(); err != nil {
			return err
		}
		l.logFile = nil
	}

	return nil
}

// diffConfigs 比较两个配置并找出差异
func (l *ConfigChangeLogger) diffConfigs(oldConfig, newConfig *Config) map[string]interface{} {
	changes := make(map[string]interface{})

	// 简单比较一些常用配置项
	// 存储模式变更
	if oldConfig.Storage.Mode != newConfig.Storage.Mode {
		changes["storage.mode"] = map[string]interface{}{
			"old": oldConfig.Storage.Mode,
			"new": newConfig.Storage.Mode,
		}
	}

	// 块大小变更
	if oldConfig.Storage.BlockStrategy.BlockSize != newConfig.Storage.BlockStrategy.BlockSize {
		changes["storage.blockStrategy.blockSize"] = map[string]interface{}{
			"old": oldConfig.Storage.BlockStrategy.BlockSize,
			"new": newConfig.Storage.BlockStrategy.BlockSize,
		}
	}

	// 压缩设置变更
	if oldConfig.Storage.Compression.Enabled != newConfig.Storage.Compression.Enabled {
		changes["storage.compression.enabled"] = map[string]interface{}{
			"old": oldConfig.Storage.Compression.Enabled,
			"new": newConfig.Storage.Compression.Enabled,
		}
	}

	// 工作线程数变更
	if oldConfig.Performance.Parallelism.MaxWorkers != newConfig.Performance.Parallelism.MaxWorkers {
		changes["performance.parallelism.maxWorkers"] = map[string]interface{}{
			"old": oldConfig.Performance.Parallelism.MaxWorkers,
			"new": newConfig.Performance.Parallelism.MaxWorkers,
		}
	}

	// 直接IO变更
	if oldConfig.Performance.IO.UseDirectIO != newConfig.Performance.IO.UseDirectIO {
		changes["performance.io.useDirectIO"] = map[string]interface{}{
			"old": oldConfig.Performance.IO.UseDirectIO,
			"new": newConfig.Performance.IO.UseDirectIO,
		}
	}

	// 安全设置变更
	if oldConfig.Security.Encryption.Enabled != newConfig.Security.Encryption.Enabled {
		changes["security.encryption.enabled"] = map[string]interface{}{
			"old": oldConfig.Security.Encryption.Enabled,
			"new": newConfig.Security.Encryption.Enabled,
		}
	}

	// 索引设置变更
	if oldConfig.Index.Enabled != newConfig.Index.Enabled {
		changes["index.enabled"] = map[string]interface{}{
			"old": oldConfig.Index.Enabled,
			"new": newConfig.Index.Enabled,
		}
	}

	// 可以添加更多的差异比较

	return changes
}
