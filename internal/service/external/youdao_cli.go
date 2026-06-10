package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// YoudaoNoteItem 有道云笔记列表项
type YoudaoNoteItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "file" 或 "dir"
	ParentID string `json:"parentId,omitempty"`
}

// YoudaoReadResult 有道云笔记读取结果
type YoudaoReadResult struct {
	Content   string `json:"content"`
	RawFormat string `json:"rawFormat"` // md, note, txt
	IsRaw     bool   `json:"isRaw"`
}

// YoudaoCLI 有道云笔记 CLI 接口
type YoudaoCLI interface {
	// CheckAvailable 检查 CLI 是否可用
	CheckAvailable() error
	// List 列出目录下笔记（根目录传空字符串）
	List(apiKey string, folderID string) ([]YoudaoNoteItem, error)
	// Read 读取笔记内容
	Read(apiKey string, fileID string) (*YoudaoReadResult, error)
	// Search 搜索笔记
	Search(apiKey string, keyword string) ([]YoudaoNoteItem, error)
	// CreateNote 创建笔记
	CreateNote(apiKey string, title string, content string, parentID string) (string, error)
	// UpdateNote 更新笔记内容
	UpdateNote(apiKey string, fileID string, content string) error
	// DeleteNote 删除笔记
	DeleteNote(apiKey string, fileID string) error
}

// youdaoCLI YoudaoCLI 实现
type youdaoCLI struct {
	cliPath string
}

// NewYoudaoCLI 创建 YoudaoCLI 实例
func NewYoudaoCLI(cliPath string) YoudaoCLI {
	if cliPath == "" {
		cliPath = "youdaonote"
	}
	return &youdaoCLI{cliPath: cliPath}
}

// runWithKey 执行 CLI 命令，通过临时 HOME 目录隔离用户 API Key
func (c *youdaoCLI) runWithKey(apiKey string, args []string, stdinData string) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "youdaonote-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 写入用户配置文件
	config := fmt.Sprintf(`{"apiKey":"%s"}`, apiKey)
	configPath := filepath.Join(tmpDir, ".youdaonote.json")
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return nil, fmt.Errorf("写入配置失败: %w", err)
	}

	fullArgs := append([]string{"-s", "ydn"}, args...)
	fullArgs = append(fullArgs, "--json")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cliPath, fullArgs...)
	cmd.Env = append(os.Environ(),
		"HOME="+tmpDir,
		"USERPROFILE="+tmpDir,
	)

	if stdinData != "" {
		cmd.Stdin = bytes.NewBufferString(stdinData)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("CLI 执行失败: %s", string(exitErr.Stderr))
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("CLI 调用超时（30s）")
		}
		return nil, fmt.Errorf("CLI 调用失败: %w", err)
	}
	return output, nil
}

// CheckAvailable 检查 CLI 是否可用
func (c *youdaoCLI) CheckAvailable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cliPath, "-s", "ydn", "list", "--json")
	cmd.Env = append(os.Environ(), "HOME="+os.TempDir(), "USERPROFILE="+os.TempDir())

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("youdaonote CLI 调用超时")
		}
		outputStr := string(output)
		if strings.Contains(outputStr, "command not found") || strings.Contains(outputStr, "not found") {
			return fmt.Errorf("youdaonote CLI 未安装")
		}
		// CLI 存在但可能返回错误（如 API Key 未配置），这不算不可用
		if strings.Contains(outputStr, "apiKey") || strings.Contains(outputStr, "config") {
			return nil // CLI 存在，只是未配置 Key
		}
		return fmt.Errorf("youdaonote CLI 不可用: %w", err)
	}
	return nil
}

// List 列出目录下笔记
func (c *youdaoCLI) List(apiKey string, folderID string) ([]YoudaoNoteItem, error) {
	args := []string{"list"}
	if folderID != "" {
		args = append(args, "-f", folderID)
	}

	output, err := c.runWithKey(apiKey, args, "")
	if err != nil {
		return nil, err
	}

	var items []YoudaoNoteItem
	if err := json.Unmarshal(output, &items); err != nil {
		// 尝试解析为单个对象（CLI 有时返回单对象而非数组）
		var single YoudaoNoteItem
		if err2 := json.Unmarshal(output, &single); err2 == nil {
			return []YoudaoNoteItem{single}, nil
		}
		return nil, fmt.Errorf("解析笔记列表失败: %w", err)
	}
	return items, nil
}

// Read 读取笔记内容
func (c *youdaoCLI) Read(apiKey string, fileID string) (*YoudaoReadResult, error) {
	output, err := c.runWithKey(apiKey, []string{"read", fileID}, "")
	if err != nil {
		return nil, err
	}

	var result YoudaoReadResult
	if err := json.Unmarshal(output, &result); err != nil {
		// 降级：将原始输出作为内容
		return &YoudaoReadResult{
			Content:   string(output),
			RawFormat: "txt",
			IsRaw:     true,
		}, nil
	}
	return &result, nil
}

// Search 搜索笔记
func (c *youdaoCLI) Search(apiKey string, keyword string) ([]YoudaoNoteItem, error) {
	output, err := c.runWithKey(apiKey, []string{"search", keyword}, "")
	if err != nil {
		return nil, err
	}

	var items []YoudaoNoteItem
	if err := json.Unmarshal(output, &items); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}
	return items, nil
}

// CreateNote 创建笔记（使用 save 命令，支持 Markdown）
func (c *youdaoCLI) CreateNote(apiKey string, title string, content string, parentID string) (string, error) {
	saveData := map[string]string{
		"title":   title,
		"type":    "md",
		"content": content,
	}
	if parentID != "" {
		saveData["parentId"] = parentID
	}

	jsonBytes, err := json.Marshal(saveData)
	if err != nil {
		return "", fmt.Errorf("序列化笔记数据失败: %w", err)
	}

	output, err := c.runWithKey(apiKey, []string{"save"}, string(jsonBytes))
	if err != nil {
		return "", err
	}

	// 尝试从返回中提取笔记 ID
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err == nil {
		if id, ok := result["id"].(string); ok {
			return id, nil
		}
	}

	return string(output), nil
}

// UpdateNote 更新笔记内容
func (c *youdaoCLI) UpdateNote(apiKey string, fileID string, content string) error {
	_, err := c.runWithKey(apiKey, []string{"update", fileID, "-c", content}, "")
	return err
}

// DeleteNote 删除笔记
func (c *youdaoCLI) DeleteNote(apiKey string, fileID string) error {
	_, err := c.runWithKey(apiKey, []string{"delete", fileID}, "")
	return err
}
