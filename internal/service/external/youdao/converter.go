package youdao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// NoteConverter 有道云笔记格式转换器（调用 youdaonote-pull 的 Python 脚本）
type NoteConverter interface {
	// ConvertNote 将 .note 格式转换为 Markdown
	ConvertNote(fileID string, cookiesPath string) (string, error)
}

// noteConverter 实现
type noteConverter struct {
	scriptPath string
}

// NewNoteConverter 创建转换器实例
// scriptPath: convert_note.py 脚本的路径
func NewNoteConverter(scriptPath string) NoteConverter {
	return &noteConverter{scriptPath: scriptPath}
}

// convertResult Python 脚本返回的结果
type convertResult struct {
	Content string `json:"content"`
	Error   string `json:"error"`
}

// ConvertNote 调用 Python 脚本转换 .note 文件为 Markdown
func (c *noteConverter) ConvertNote(fileID string, cookiesPath string) (string, error) {
	// 检查脚本是否存在
	if _, err := os.Stat(c.scriptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("转换脚本不存在: %s", c.scriptPath)
	}

	// 检查 cookies 文件是否存在
	if _, err := os.Stat(cookiesPath); os.IsNotExist(err) {
		return "", fmt.Errorf("cookies 文件不存在: %s", cookiesPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 获取脚本所在目录（用于设置工作目录）
	scriptDir := filepath.Dir(c.scriptPath)

	// 构建命令
	cmd := exec.CommandContext(ctx, "python", c.scriptPath, fileID, cookiesPath)
	cmd.Dir = scriptDir

	// 执行命令
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("转换超时（60s）")
		}
		// 尝试从 stderr 获取错误信息
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("转换失败: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("转换失败: %w", err)
	}

	// 解析结果
	var result convertResult
	if err := json.Unmarshal(output, &result); err != nil {
		// 如果不是 JSON，可能是直接输出的内容
		content := strings.TrimSpace(string(output))
		if content == "" {
			return "", fmt.Errorf("转换结果为空")
		}
		return content, nil
	}

	if result.Error != "" {
		return "", errors.New(result.Error)
	}

	if result.Content == "" {
		return "", fmt.Errorf("转换后内容为空")
	}

	return result.Content, nil
}
