package rag

import (
	"context"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// ASTDocument 解析后的文档结构（内部使用）
type ASTDocument struct {
	Sections []*Section
}

// Section 文档章节
type Section struct {
	Heading     string
	Level       int    // 1=H1, 2=H2, ...
	ChapterPath string // 完整路径，如 "第一章 > 1.1 背景"
	Content     []ContentBlock
}

// ContentBlock 内容块
type ContentBlock struct {
	Type     string // paragraph/code/table/image/mermaid/quote
	Content  string
	Language string // 代码块语言（仅 code 类型使用）
}

// MarkdownParser Markdown AST 解析器，实现 eino parser.Parser 接口
type MarkdownParser struct {
	md goldmark.Markdown
}

// NewMarkdownParser 创建 Markdown 解析器
func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{
		md: goldmark.New(),
	}
}

// Parse 实现 eino parser.Parser 接口。
// 将 Markdown io.Reader 解析为 []*schema.Document。
// 每个章节生成一个包含结构信息的文档。
func (p *MarkdownParser) Parse(ctx context.Context, reader io.Reader, opts ...parser.Option) ([]*schema.Document, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	doc := p.md.Parser().Parse(text.NewReader(content))
	astDoc := p.buildASTDocument(doc, content)

	var docs []*schema.Document
	for i, section := range astDoc.Sections {
		d := &schema.Document{
			ID:      "",
			Content: section.buildContent(),
			MetaData: map[string]any{
				"heading":      section.Heading,
				"level":        section.Level,
				"chapter_path": section.ChapterPath,
				"section_idx":  i,
			},
		}
		docs = append(docs, d)
	}
	return docs, nil
}

// buildASTDocument 遍历 goldmark AST，构建结构化文档
func (p *MarkdownParser) buildASTDocument(doc ast.Node, source []byte) *ASTDocument {
	result := &ASTDocument{}
	var currentSection *Section
	var headingStack []string

	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch n.Kind() {
		case ast.KindHeading:
			heading := n.(*ast.Heading)
			headingText := string(n.Text(source))
			level := heading.Level

			if level <= len(headingStack) {
				headingStack = headingStack[:level-1]
			}
			headingStack = append(headingStack, headingText)

			currentSection = &Section{
				Heading:     headingText,
				Level:       level,
				ChapterPath: joinPath(headingStack),
			}
			result.Sections = append(result.Sections, currentSection)

		default:
			if currentSection == nil {
				currentSection = &Section{
					Heading:     "",
					Level:       0,
					ChapterPath: "",
				}
				result.Sections = append(result.Sections, currentSection)
			}
			block := p.extractContentBlock(n, source)
			if block != nil {
				currentSection.Content = append(currentSection.Content, *block)
			}
		}
	}
	return result
}

// extractContentBlock 从 AST 节点提取内容块
func (p *MarkdownParser) extractContentBlock(n ast.Node, source []byte) *ContentBlock {
	switch n.Kind() {
	case ast.KindParagraph:
		return &ContentBlock{Type: "paragraph", Content: string(n.Text(source))}
	case ast.KindFencedCodeBlock:
		lang := ""
		codeBlock := n.(*ast.FencedCodeBlock)
		if codeBlock.Info != nil {
			lang = string(codeBlock.Info.Text(source))
		}
		var code []byte
		lines := codeBlock.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			code = append(code, seg.Value(source)...)
			code = append(code, '\n')
		}
		if lang == "mermaid" {
			return &ContentBlock{Type: "mermaid", Content: string(code)}
		}
		return &ContentBlock{Type: "code", Content: string(code), Language: lang}
	case ast.KindBlockquote:
		return &ContentBlock{Type: "quote", Content: string(n.Text(source))}
	case ast.KindList:
		return &ContentBlock{Type: "paragraph", Content: string(n.Text(source))}
	default:
		nodeText := string(n.Text(source))
		if strings.TrimSpace(nodeText) == "" {
			return nil
		}
		return &ContentBlock{Type: "paragraph", Content: nodeText}
	}
}

// buildContent 将章节的 ContentBlock 列表合并为纯文本
// 代码块和 mermaid 会重新添加 ``` 标记，保持类型信息
func (s *Section) buildContent() string {
	var parts []string
	for _, block := range s.Content {
		switch block.Type {
		case "code":
			// 重新添加代码块标记
			parts = append(parts, "```"+block.Language+"\n"+block.Content+"```")
		case "mermaid":
			// 重新添加 mermaid 标记
			parts = append(parts, "```mermaid\n"+block.Content+"```")
		default:
			parts = append(parts, block.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}

// joinPath 将标题栈连接为章节路径
func joinPath(stack []string) string {
	return strings.Join(stack, " > ")
}
