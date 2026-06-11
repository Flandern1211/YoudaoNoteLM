package external

import (
	"bufio"
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
	// ConvertNote 将 .note 格式转换为 Markdown（需要 cookiesPath）
	ConvertNote(fileID string, cookiesPath string) (string, error)
}

// youdaoCLI YoudaoCLI 实现
type youdaoCLI struct {
	cliPath   string
	converter YoudaoNoteConverter
}

// NewYoudaoCLI 创建 YoudaoCLI 实例
func NewYoudaoCLI(cliPath string, converterScriptPath string) YoudaoCLI {
	if cliPath == "" {
		cliPath = "youdaonote"
	}
	var converter YoudaoNoteConverter
	if converterScriptPath != "" {
		converter = NewYoudaoNoteConverter(converterScriptPath)
	}
	return &youdaoCLI{
		cliPath:   cliPath,
		converter: converter,
	}
}

// youdaonoteConfig CLI 配置文件结构
type youdaonoteConfig struct {
	Backend string            `json:"backend"`
	MCP     youdaonoteMCP     `json:"mcp"`
}

type youdaonoteMCP struct {
	Server string `json:"server"`
	APIKey string `json:"apiKey"`
}

// runWithKey 执行 CLI 命令，通过临时 HOME 目录隔离用户 API Key
// CLI 读取 ~/.youdaonote.json 配置文件获取 API Key
func (c *youdaoCLI) runWithKey(apiKey string, args []string) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "youdaonote-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 写入临时配置文件（CLI 读取 ~/.youdaonote.json）
	cfg := youdaonoteConfig{
		Backend: "mcp",
		MCP: youdaonoteMCP{
			Server: "https://open.mail.163.com/api/ynote/mcp/sse",
			APIKey: apiKey,
		},
	}
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}
	configPath := filepath.Join(tmpDir, ".youdaonote.json")
	if err := os.WriteFile(configPath, cfgBytes, 0600); err != nil {
		return nil, fmt.Errorf("写入配置失败: %w", err)
	}

	// 构建命令参数：youdaonote -s ydn <args>
	fullArgs := append([]string{"-s", "ydn"}, args...)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.cliPath, fullArgs...)
	// 通过 HOME/USERPROFILE 环境变量让 CLI 读取临时目录下的配置
	// 同时保留 PATH 等系统环境变量
	cmd.Env = append(os.Environ(),
		"HOME="+tmpDir,
		"USERPROFILE="+tmpDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("CLI 调用超时（60s）")
		}
		// 输出中包含错误信息，一起返回
		outputStr := string(output)
		if outputStr != "" {
			return nil, fmt.Errorf("CLI 执行失败: %s", strings.TrimSpace(outputStr))
		}
		return nil, fmt.Errorf("CLI 调用失败: %w", err)
	}
	return output, nil
}

// CheckAvailable 检查 CLI 是否可用
func (c *youdaoCLI) CheckAvailable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 使用 check --json 检查 CLI 是否可用（不需要 API Key）
	cmd := exec.CommandContext(ctx, c.cliPath, "-s", "ydn", "check", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("youdaonote CLI 调用超时")
		}
		outputStr := string(output)
		if strings.Contains(outputStr, "command not found") || strings.Contains(outputStr, "not found") ||
			strings.Contains(outputStr, "no such file") {
			return fmt.Errorf("youdaonote CLI 未安装")
		}
		// CLI 存在但 check 失败（如配置问题），不算不可用
		if len(output) > 0 {
			return nil
		}
		return fmt.Errorf("youdaonote CLI 不可用: %w", err)
	}
	return nil
}

// parseListOutput 解析 list 命令的纯文本输出
// 实际输出格式（ID 和名称用 Tab 分隔）：
//
//	SVR459F9DAFF051431F8428974D33FFF091\t我的资源
//	2653FFE363B84B8695852F4F5CE2E3D3\ttest1.note
//
// 也支持旧格式：
//
//	📁 目录名 (id: xxx)
//	📄 笔记名 (id: yyy)
func parseListOutput(output string) []YoudaoNoteItem {
	items := make([]YoudaoNoteItem, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		item := YoudaoNoteItem{}

		// 尝试解析 Tab 分隔格式：[emoji] ID\tName
		if strings.Contains(line, "\t") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 {
				idPart := strings.TrimSpace(parts[0])
				item.Name = strings.TrimSpace(parts[1])
				// 移除 ID 前面的 emoji 前缀
				idPart = strings.TrimPrefix(idPart, "📁")
				idPart = strings.TrimPrefix(idPart, "📄")
				idPart = strings.TrimSpace(idPart)
				item.ID = idPart
				// 根据文件扩展名或 emoji 判断类型
				if strings.HasPrefix(parts[0], "📄") || strings.HasSuffix(item.Name, ".note") || strings.HasSuffix(item.Name, ".md") || strings.HasSuffix(item.Name, ".txt") {
					item.Type = "file"
				} else {
					item.Type = "dir"
				}
			}
		} else if strings.HasPrefix(line, "📁") {
			// 解析旧格式目录：📁 xxx (id: yyy)
			item.Type = "dir"
			line = strings.TrimPrefix(line, "📁")
			line = strings.TrimSpace(line)
			if idx := strings.LastIndex(line, "(id: "); idx > 0 {
				idPart := line[idx+5:]
				idPart = strings.TrimSuffix(idPart, ")")
				item.ID = strings.TrimSpace(idPart)
				item.Name = strings.TrimSpace(line[:idx])
			} else {
				item.Name = line
			}
		} else if strings.HasPrefix(line, "📄") {
			// 解析旧格式文件：📄 xxx (id: yyy)
			item.Type = "file"
			line = strings.TrimPrefix(line, "📄")
			line = strings.TrimSpace(line)
			if idx := strings.LastIndex(line, "(id: "); idx > 0 {
				idPart := line[idx+5:]
				idPart = strings.TrimSuffix(idPart, ")")
				item.ID = strings.TrimSpace(idPart)
				item.Name = strings.TrimSpace(line[:idx])
			} else {
				item.Name = line
			}
		} else if strings.HasPrefix(line, "❌") || strings.HasPrefix(line, "⚠️") {
			// 跳过错误/警告行
			continue
		} else {
			// 跳过非条目行（如标题、分隔符等）
			continue
		}

		if item.ID != "" || item.Name != "" {
			items = append(items, item)
		}
	}
	return items
}

// List 列出目录下笔记
func (c *youdaoCLI) List(apiKey string, folderID string) ([]YoudaoNoteItem, error) {
	args := []string{"list"}
	if folderID != "" {
		args = append(args, "-f", folderID)
	}

	output, err := c.runWithKey(apiKey, args)
	if err != nil {
		return nil, err
	}

	items := parseListOutput(string(output))
	return items, nil
}

// Read 读取笔记内容
func (c *youdaoCLI) Read(apiKey string, fileID string) (*YoudaoReadResult, error) {
	output, err := c.runWithKey(apiKey, []string{"read", fileID})
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(string(output))

	// 检查是否是 JSON 格式的响应（可能包含 null content）
	var jsonResp struct {
		FileID  string      `json:"fileId"`
		Content interface{} `json:"content"`
		Title   string      `json:"title"`
		Raw     bool        `json:"raw"`
	}
	if err := json.Unmarshal(output, &jsonResp); err == nil {
		// 是 JSON 响应，检查 content 是否为 null
		if jsonResp.Content == nil {
			return &YoudaoReadResult{
				Content:   "",
				RawFormat: "note",
				IsRaw:     jsonResp.Raw,
			}, nil
		}
		// content 不为 nil，转为字符串
		if contentStr, ok := jsonResp.Content.(string); ok {
			return &YoudaoReadResult{
				Content:   contentStr,
				RawFormat: "note",
				IsRaw:     jsonResp.Raw,
			}, nil
		}
	}

	// 普通文本响应
	return &YoudaoReadResult{
		Content:   content,
		RawFormat: "md",
		IsRaw:     false,
	}, nil
}

// Search 搜索笔记
func (c *youdaoCLI) Search(apiKey string, keyword string) ([]YoudaoNoteItem, error) {
	output, err := c.runWithKey(apiKey, []string{"search", keyword})
	if err != nil {
		return nil, err
	}

	items := parseListOutput(string(output))
	return items, nil
}

// CreateNote 创建笔记（使用 save 命令，支持 Markdown）
func (c *youdaoCLI) CreateNote(apiKey string, title string, content string, parentID string) (string, error) {
	// 构建 save 命令的 JSON 参数
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

	// 将 JSON 写入临时文件，用 --file 参数传递（避免 Windows 管道编码问题）
	tmpFile, err := os.CreateTemp("", "youdaonote-save-*.json")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(jsonBytes); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()

	output, err := c.runWithKey(apiKey, []string{"save", "--json", "--file", tmpFile.Name()})
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

	// 降级：返回原始输出
	return strings.TrimSpace(string(output)), nil
}

// UpdateNote 更新笔记内容
func (c *youdaoCLI) UpdateNote(apiKey string, fileID string, content string) error {
	// 将内容写入临时文件，用 --file 传递（避免 Windows 编码问题）
	tmpFile, err := os.CreateTemp("", "youdaonote-update-*.md")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()

	_, err = c.runWithKey(apiKey, []string{"update", fileID, "--file", tmpFile.Name()})
	return err
}

// DeleteNote 删除笔记
func (c *youdaoCLI) DeleteNote(apiKey string, fileID string) error {
	_, err := c.runWithKey(apiKey, []string{"delete", fileID})
	return err
}

// ConvertNote 将 .note 格式转换为 Markdown（使用 youdaonote-pull 的 Python 脚本）
func (c *youdaoCLI) ConvertNote(fileID string, cookiesPath string) (string, error) {
	if c.converter == nil {
		return "", fmt.Errorf("转换器未初始化，请配置 converter_script_path")
	}
	return c.converter.ConvertNote(fileID, cookiesPath)
}
