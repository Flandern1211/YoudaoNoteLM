# 导入模块、搜索Agent模块、后台模块 技术设计文档

> 版本：V1.0
> 日期：2026-06-04
> 状态：草稿
> 负责模块：导入模块、搜索Agent模块、后台模块

---

## 1. 整体架构设计

### 1.1 系统架构图

```
┌─────────────────────────────────────────────────────────────┐
│                      Frontend (Vite + Vue)                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │ 导入面板  │  │ 资料列表  │  │ AI 对话  │  │ 笔记管理 │    │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────────┘    │
└───────┼──────────────┼──────────────┼────────────────────────┘
        │              │              │
        ▼              ▼              ▼
┌─────────────────────────────────────────────────────────────┐
│                    API Gateway (Gin)                         │
│  /api/v1/sources/*   /api/v1/import/*   /api/v1/admin/*     │
└───────┬──────────────┬──────────────┬────────────────────────┘
        │              │              │
        ▼              ▼              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Service Layer                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │ SourceSvc│  │ImporterSvc│ │SearchAgent│  │ AdminSvc │    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │
└───────┬──────────────┬──────────────┬────────────────────────┘
        │              │              │
        ▼              ▼              ▼
┌─────────────────────────────────────────────────────────────┐
│                   External Services                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │MarkItDown│  │Whisper   │  │DuckDuckGo│  │YoudaoNote│    │
│  │(文件解析) │  │(语音转写) │  │(内置搜索) │  │Skill/CLI │    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │
└─────────────────────────────────────────────────────────────┘
        │              │              │
        ▼              ▼              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Data Layer                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                   │
│  │  MySQL   │  │  Milvus  │  │  MinIO   │                   │
│  │(结构化数据)│  │(向量数据) │  │(文件存储) │                   │
│  └──────────┘  └──────────┘  └──────────┘                   │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 模块职责划分

| 模块 | 职责 | 核心接口 |
|------|------|----------|
| **Source（资料来源）** | CRUD 管理、搜索过滤、原格式查看 | `SourceService` |
| **Importer（导入）** | 文件/音频/搜索结果导入、调用 MarkItDown/Whisper | `ImporterService` |
| **SearchAgent（搜索Agent）** | 理解搜索意图、调用搜索引擎、URL直接导入 | `SearchAgentService` |
| **YoudaoAgent（有道Agent）** | 有道云笔记授权、浏览、导入 | `YoudaoAgentService` |
| **Admin（后台）** | 用户管理、系统配置（ASR/搜索/Embedding） | `AdminService` |

**模块边界说明**：
- 本模块负责：导入 + 来源管理 + 搜索Agent + 后台配置
- 其他模块负责：向量化（Embedding）+ 语义检索（RAG）+ AI 对话 + 内容生成
- 导入模块通过 `EmbeddingService` 接口调用向量化服务，不实现向量化逻辑

### 1.3 技术栈

| 组件 | 选型 | 说明 |
|------|------|------|
| Web 框架 | Gin | 当前项目已有 |
| ORM | GORM | 当前项目已有 |
| Agent 框架 | Eino | 搜索Agent和有道Agent |
| 数据库 | MySQL | 结构化数据 |
| 向量数据库 | Milvus | 向量数据（由其他模块管理） |
| 文件存储 | MinIO | 原始文件存储 |
| 文件解析 | MarkItDown HTTP 服务 | 已部署 |
| ASR | OpenAI Whisper (Python) | 内置免费方案 |
| 网络搜索 | DuckDuckGo | 内置免费方案 |

### 1.4 数据流

**文件导入流程：**
```
用户上传文件 → API接收 → 存储原始文件(MinIO) → 调用MarkItDown转Markdown
→ 保存Source记录(MySQL) → 调用EmbeddingService触发切片+向量化 → 返回结果
```

**网络搜索导入流程：**
```
用户输入关键词/URL → SearchAgent(Eino理解意图) → 调用搜索引擎
→ 返回搜索结果列表 → 用户选择 → 调用MarkItDown抓取网页 → 保存Source → 向量化
```

**音频导入流程（含预览）：**
```
用户上传音频 → 调用Whisper转写文本 → 返回预览给用户
→ 用户编辑/确认 → 调用MarkItDown转Markdown → 保存Source → 向量化
```

---

## 2. 数据库设计

### 2.1 MySQL 表结构

#### source（资料来源表）

```sql
CREATE TABLE `source` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT UNSIGNED NOT NULL COMMENT '所属用户',
    `notebook_id`   BIGINT UNSIGNED NOT NULL COMMENT '所属笔记本',
    `name`          VARCHAR(255) NOT NULL COMMENT '来源名称',
    `type`          VARCHAR(20) NOT NULL COMMENT '类型: file/url/audio/note/youdao',
    `original_url`  VARCHAR(2048) DEFAULT NULL COMMENT '原始URL（网址导入时）',
    `file_path`     VARCHAR(512) DEFAULT NULL COMMENT 'MinIO文件路径',
    `file_size`     BIGINT DEFAULT 0 COMMENT '文件大小(字节)',
    `mime_type`     VARCHAR(100) DEFAULT NULL COMMENT 'MIME类型',
    `markdown_content` LONGTEXT DEFAULT NULL COMMENT '解析后的Markdown内容',
    `raw_content`   LONGTEXT DEFAULT NULL COMMENT '原始内容（用于切换显示）',
    `status`        VARCHAR(20) NOT NULL DEFAULT 'pending' COMMENT '状态: pending/processing/ready/failed',
    `error_message` VARCHAR(512) DEFAULT NULL COMMENT '失败原因',
    `is_source`     TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已添加为资料来源（笔记专用）',
    `vectorized`    TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已向量化',
    `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `deleted_at`    DATETIME DEFAULT NULL,
    PRIMARY KEY (`id`),
    INDEX `idx_user_notebook` (`user_id`, `notebook_id`),
    INDEX `idx_status` (`status`),
    INDEX `idx_type` (`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**字段说明：**
- `type`: 区分导入来源类型
  - `file`: 本地文件导入
  - `url`: 网址导入（通过搜索Agent）
  - `audio`: 音频导入
  - `note`: 笔记转资料来源
  - `youdao`: 有道云笔记导入
- `status`: 导入状态流转
  - `pending` → `processing` → `ready`
  - `pending` → `processing` → `failed`
- `markdown_content`: 所有类型统一转为 Markdown 存储
- `raw_content`: 保留原始内容用于切换显示（音频不存原始内容，URL 存原始链接）

#### import_task（导入任务表）

用于批量导入场景（搜索结果多选导入、有道笔记批量导入），跟踪进度和部分失败。

```sql
CREATE TABLE `import_task` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT UNSIGNED NOT NULL,
    `notebook_id`   BIGINT UNSIGNED NOT NULL,
    `task_type`     VARCHAR(20) NOT NULL COMMENT '任务类型: search/youdao',
    `total_count`   INT NOT NULL DEFAULT 0 COMMENT '总数量',
    `success_count` INT NOT NULL DEFAULT 0 COMMENT '成功数量',
    `fail_count`    INT NOT NULL DEFAULT 0 COMMENT '失败数量',
    `status`        VARCHAR(20) NOT NULL DEFAULT 'pending' COMMENT '状态: pending/running/completed/partial_failed',
    `error_detail`  JSON DEFAULT NULL COMMENT '失败详情',
    `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### audio_preview（音频预览表）

存储音频转写预览，等待用户确认导入。

```sql
CREATE TABLE `audio_preview` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `preview_id`    VARCHAR(36) NOT NULL COMMENT 'UUID，用于前端标识',
    `user_id`       BIGINT UNSIGNED NOT NULL,
    `notebook_id`   BIGINT UNSIGNED NOT NULL,
    `file_name`     VARCHAR(255) NOT NULL COMMENT '原始文件名',
    `file_path`     VARCHAR(512) NOT NULL COMMENT 'MinIO文件路径',
    `file_size`     BIGINT DEFAULT 0,
    `transcribed_text` LONGTEXT NOT NULL COMMENT 'Whisper转写结果',
    `status`        VARCHAR(20) NOT NULL DEFAULT 'pending' COMMENT 'pending/confirmed/expired',
    `expires_at`    DATETIME NOT NULL COMMENT '过期时间（30分钟）',
    `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_preview_id` (`preview_id`),
    INDEX `idx_user` (`user_id`),
    INDEX `idx_expires` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### sys_config（系统配置表）

动态配置表，不硬编码可配置项，通过 `config_group` 动态加载。

```sql
CREATE TABLE `sys_config` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `config_group`  VARCHAR(50) NOT NULL COMMENT '配置组: asr/search/embedding',
    `config_key`    VARCHAR(100) NOT NULL,
    `config_value`  JSON NOT NULL COMMENT '配置值',
    `enabled`       TINYINT(1) NOT NULL DEFAULT 1,
    `description`   VARCHAR(255) DEFAULT NULL COMMENT '配置描述',
    `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_group_key` (`config_group`, `config_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**config_group 说明：**
- `asr`: ASR 服务配置（provider, model_path, language 等）
- `search`: 搜索服务配置（provider, daily_quota 等）
- `embedding`: Embedding 服务配置（provider, model, dimensions 等）
- 后续可扩展新的配置组，代码无需修改

**示例数据：**
```json
{
    "config_group": "asr",
    "config_key": "whisper_local",
    "config_value": {
        "provider": "whisper_local",
        "model_path": "/models/whisper-base",
        "language": "zh"
    },
    "enabled": true
}
```

#### user_search_config（用户搜索配置）

```sql
CREATE TABLE `user_search_config` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT UNSIGNED NOT NULL,
    `name`          VARCHAR(100) NOT NULL COMMENT '配置名称',
    `provider`      VARCHAR(50) NOT NULL COMMENT '提供商: google/bing/serpapi',
    `api_key`       VARCHAR(512) NOT NULL COMMENT 'API Key（加密存储）',
    `api_url`       VARCHAR(512) DEFAULT NULL,
    `daily_quota`   INT DEFAULT NULL COMMENT '每日配额(NULL为无限)',
    `quota_used`    INT NOT NULL DEFAULT 0 COMMENT '今日已用',
    `quota_reset_at` DATE DEFAULT NULL COMMENT '配额重置日期',
    `enabled`       TINYINT(1) NOT NULL DEFAULT 1,
    `priority`      INT NOT NULL DEFAULT 0 COMMENT '优先级（数字越小越优先）',
    `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### user_asr_config（用户ASR配置）

```sql
CREATE TABLE `user_asr_config` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT UNSIGNED NOT NULL,
    `name`          VARCHAR(100) NOT NULL,
    `provider`      VARCHAR(50) NOT NULL COMMENT '提供商: whisper_api/fun_asr',
    `api_key`       VARCHAR(512) DEFAULT NULL,
    `api_url`       VARCHAR(512) NOT NULL,
    `extra_config`  JSON DEFAULT NULL COMMENT '额外参数',
    `enabled`       TINYINT(1) NOT NULL DEFAULT 1,
    `priority`      INT NOT NULL DEFAULT 0,
    `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### user_embedding_config（用户Embedding配置）

```sql
CREATE TABLE `user_embedding_config` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT UNSIGNED NOT NULL,
    `name`          VARCHAR(100) NOT NULL,
    `provider`      VARCHAR(50) NOT NULL COMMENT '提供商: openai/local',
    `api_key`       VARCHAR(512) DEFAULT NULL,
    `api_url`       VARCHAR(512) NOT NULL,
    `model_name`    VARCHAR(100) DEFAULT NULL,
    `dimensions`    INT DEFAULT NULL COMMENT '向量维度',
    `enabled`       TINYINT(1) NOT NULL DEFAULT 1,
    `priority`      INT NOT NULL DEFAULT 0,
    `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### youdao_binding（有道云笔记绑定表）

```sql
CREATE TABLE `youdao_binding` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT UNSIGNED NOT NULL,
    `api_key`       VARCHAR(512) NOT NULL COMMENT 'API Key（加密存储）',
    `status`        VARCHAR(20) NOT NULL DEFAULT 'active' COMMENT '状态: active/expired/revoked',
    `created_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at`    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 2.2 实体定义

```go
// Source 资料来源实体
type Source struct {
    entity.BaseModel
    UserID          uint   `gorm:"not null;index:idx_user_notebook"`
    NotebookID      uint   `gorm:"not null;index:idx_user_notebook"`
    Name            string `gorm:"size:255;not null"`
    Type            string `gorm:"size:20;not null;index:idx_type"`
    OriginalURL     string `gorm:"size:2048"`
    FilePath        string `gorm:"size:512"`
    FileSize        int64
    MimeType        string `gorm:"size:100"`
    MarkdownContent string `gorm:"type:longtext"`
    RawContent      string `gorm:"type:longtext"`
    Status          string `gorm:"size:20;default:pending;index:idx_status"`
    ErrorMessage    string `gorm:"size:512"`
    IsSource        bool   `gorm:"default:false"`
    Vectorized      bool   `gorm:"default:false"`
}

// ImportTask 导入任务实体
type ImportTask struct {
    entity.BaseModel
    UserID       uint   `gorm:"not null;index"`
    NotebookID   uint   `gorm:"not null"`
    TaskType     string `gorm:"size:20;not null"`
    TotalCount   int
    SuccessCount int
    FailCount    int
    Status       string `gorm:"size:20;default:pending"`
    ErrorDetail  string `gorm:"type:json"`
}

// AudioPreview 音频预览实体
type AudioPreview struct {
    entity.BaseModel
    PreviewID       string    `gorm:"size:36;not null;uniqueIndex:uk_preview_id"`
    UserID          uint      `gorm:"not null;index"`
    NotebookID      uint      `gorm:"not null"`
    FileName        string    `gorm:"size:255;not null"`
    FilePath        string    `gorm:"size:512;not null"`
    FileSize        int64
    TranscribedText string    `gorm:"type:longtext;not null"`
    Status          string    `gorm:"size:20;default:pending"`
    ExpiresAt       time.Time `gorm:"not null;index:idx_expires"`
}

// SysConfig 系统配置实体
type SysConfig struct {
    entity.BaseModel
    ConfigGroup string `gorm:"size:50;not null;uniqueIndex:uk_group_key"`
    ConfigKey   string `gorm:"size:100;not null;uniqueIndex:uk_group_key"`
    ConfigValue string `gorm:"type:json;not null"`
    Enabled     bool   `gorm:"default:true"`
    Description string `gorm:"size:255"`
}

// YoudaoBinding 有道云笔记绑定实体
type YoudaoBinding struct {
    entity.BaseModel
    UserID  uint   `gorm:"not null;uniqueIndex:uk_user"`
    APIKey  string `gorm:"size:512;not null"`
    Status  string `gorm:"size:20;default:active"`
}
```

---

## 3. API 接口设计

### 3.1 资料来源管理 API

| 方法 | 路径 | 说明 | 请求参数 | 响应 |
|------|------|------|----------|------|
| GET | `/api/v1/notebooks/:nbId/sources` | 获取来源列表 | `?keyword=&page=&size=` | `{sources: [], total: int}` |
| GET | `/api/v1/sources/:id` | 获取来源详情 | - | `{source: Source}` |
| PUT | `/api/v1/sources/:id` | 重命名来源 | `{name: string}` | `{source: Source}` |
| DELETE | `/api/v1/sources/:id` | 删除来源 | - | `{message: string}` |
| POST | `/api/v1/sources/batch-delete` | 批量删除 | `{ids: [uint]}` | `{message: string}` |
| GET | `/api/v1/sources/:id/content` | 获取Markdown内容 | - | `{content: string}` |
| GET | `/api/v1/sources/:id/original` | 获取原格式内容 | - | `{content: string, type: string}` |

**原格式内容说明：**
- `file` 类型：返回文件渲染内容（PDF/DOCX等）
- `url` 类型：返回原始 URL 链接
- `audio` 类型：不支持查看原格式
- `note`/`youdao` 类型：返回原始 Markdown

### 3.2 导入 API

| 方法 | 路径 | 说明 | 请求参数 | 响应 |
|------|------|------|----------|------|
| POST | `/api/v1/notebooks/:nbId/import/file` | 文件上传导入（直接） | `multipart: file` | `{source: Source}` |
| POST | `/api/v1/notebooks/:nbId/import/audio/preview` | 音频上传转写预览 | `multipart: file` | `{preview_id, content, file_name}` |
| POST | `/api/v1/import/audio/confirm` | 确认音频导入 | `{preview_id, content?, notebook_id}` | `{source: Source}` |
| GET | `/api/v1/import/tasks/:taskId` | 查询导入任务进度 | - | `{task: ImportTask}` |

**文件导入请求：**
```
Content-Type: multipart/form-data
Fields: file (单文件, ≤30MB, 支持 .txt/.md/.docx/.pdf/.pptx)
```

**音频预览请求：**
```
Content-Type: multipart/form-data
Fields: file (单文件, .mp3/.wav, ≤300MB)
```

**音频预览响应：**
```json
{
    "preview_id": "550e8400-e29b-41d4-a716-446655440000",
    "content": "转写后的文本内容...",
    "file_name": "meeting.mp3"
}
```

**音频确认导入请求：**
```json
{
    "preview_id": "550e8400-e29b-41d4-a716-446655440000",
    "content": "用户编辑后的内容（可选，为空则用原始转写内容）",
    "notebook_id": 1
}
```

### 3.3 搜索 Agent API

| 方法 | 路径 | 说明 | 请求参数 | 响应 |
|------|------|------|----------|------|
| POST | `/api/v1/notebooks/:nbId/search` | 搜索/URL导入 | `{query: string, type: "keyword"\|"url"}` | `{results: []}` 或 `{source: Source}` |
| POST | `/api/v1/notebooks/:nbId/search/import` | 批量导入搜索结果 | `{urls: [string]}` | `{task: ImportTask}` |

**搜索请求（关键词）：**
```json
{
    "query": "机器学习入门",
    "type": "keyword"
}
```
响应：
```json
{
    "results": [
        {
            "title": "机器学习入门指南",
            "url": "https://example.com/ml-guide",
            "snippet": "本文介绍机器学习的基本概念..."
        }
    ]
}
```

**搜索请求（URL）：**
```json
{
    "query": "https://example.com/article",
    "type": "url"
}
```
响应：直接返回导入后的 Source 对象

### 3.4 有道云笔记 API

| 方法 | 路径 | 说明 | 请求参数 | 响应 |
|------|------|------|----------|------|
| POST | `/api/v1/youdao/bind` | 绑定有道云笔记 | `{api_key: string}` | `{message: string}` |
| DELETE | `/api/v1/youdao/unbind` | 解除绑定 | - | `{message: string}` |
| GET | `/api/v1/youdao/status` | 查询绑定状态 | - | `{bound: bool, status: string}` |
| GET | `/api/v1/youdao/notes` | 浏览笔记列表 | `?page=&size=&keyword=` | `{notes: [], total: int}` |
| POST | `/api/v1/youdao/notes/preview` | 预览笔记内容 | `{note_id: string}` | `{content: string}` |
| POST | `/api/v1/youdao/notes/import` | 批量导入笔记 | `{note_ids: [string]}` | `{task: ImportTask}` |

### 3.5 后台管理 API

| 方法 | 路径 | 说明 | 请求参数 | 响应 |
|------|------|------|----------|------|
| GET | `/api/v1/admin/users` | 用户列表 | `?keyword=&page=&size=` | `{users: [], total: int}` |
| PUT | `/api/v1/admin/users/:id/status` | 启用/禁用用户 | `{enabled: bool}` | `{message: string}` |
| GET | `/api/v1/admin/config/:group` | 获取配置组 | - | `{configs: []}` |
| PUT | `/api/v1/admin/config/:group/:key` | 更新配置 | `{config_value: json, enabled: bool}` | `{message: string}` |
| POST | `/api/v1/admin/config/:group` | 新增配置 | `{config_key, config_value, description}` | `{config: SysConfig}` |
| GET | `/api/v1/admin/config/status` | 所有服务配置状态汇总 | - | `{groups: []}` |

**配置状态汇总响应：**
```json
{
    "groups": [
        {
            "group": "asr",
            "total": 2,
            "enabled": 1,
            "description": "语音转文本服务"
        },
        {
            "group": "search",
            "total": 1,
            "enabled": 1,
            "description": "网络搜索服务"
        },
        {
            "group": "embedding",
            "total": 1,
            "enabled": 1,
            "description": "向量化服务"
        }
    ]
}
```

### 3.6 用户配置 API

| 方法 | 路径 | 说明 | 请求参数 | 响应 |
|------|------|------|----------|------|
| GET | `/api/v1/user/config/search` | 获取用户搜索配置列表 | - | `{configs: []}` |
| POST | `/api/v1/user/config/search` | 新增搜索配置 | `{name, provider, api_key, api_url?, daily_quota?}` | `{config}` |
| PUT | `/api/v1/user/config/search/:id` | 更新搜索配置 | 同上 | `{config}` |
| DELETE | `/api/v1/user/config/search/:id` | 删除搜索配置 | - | `{message: string}` |
| GET | `/api/v1/user/config/asr` | 获取用户ASR配置列表 | - | `{configs: []}` |
| POST | `/api/v1/user/config/asr` | 新增ASR配置 | `{name, provider, api_url, api_key?, extra_config?}` | `{config}` |
| PUT | `/api/v1/user/config/asr/:id` | 更新ASR配置 | 同上 | `{config}` |
| DELETE | `/api/v1/user/config/asr/:id` | 删除ASR配置 | - | `{message: string}` |
| GET | `/api/v1/user/config/embedding` | 获取用户Embedding配置列表 | - | `{configs: []}` |
| POST | `/api/v1/user/config/embedding` | 新增Embedding配置 | `{name, provider, api_url, api_key?, model_name?, dimensions?}` | `{config}` |
| PUT | `/api/v1/user/config/embedding/:id` | 更新Embedding配置 | 同上 | `{config}` |
| DELETE | `/api/v1/user/config/embedding/:id` | 删除Embedding配置 | - | `{message: string}` |

### 3.7 错误响应格式

```json
{
    "code": 40001,
    "message": "不支持的文件格式",
    "details": "仅支持 .txt, .md, .docx, .pdf, .pptx 格式"
}
```

**错误码定义：**

| 错误码 | 说明 |
|--------|------|
| 40001 | 不支持的文件格式 |
| 40002 | 文件大小超限 |
| 40003 | 文件解析失败 |
| 40004 | 网页抓取失败 |
| 40005 | 音频转写失败 |
| 40006 | 搜索API配额耗尽 |
| 40007 | 有道API Key无效 |
| 40008 | 重复导入 |
| 40009 | 预览已过期 |
| 50001 | 内部服务错误 |

---

## 4. 详细设计

### 4.1 导入模块（Importer）

#### 目录结构

```
internal/
├── api/v1/source/
│   ├── controller.go
│   └── routes.go
├── api/v1/import/
│   ├── controller.go
│   └── routes.go
├── service/
│   ├── source_interface.go
│   ├── source_service.go
│   ├── importer_interface.go
│   └── importer_service.go
├── service/external/
│   ├── markitdown_interface.go
│   ├── markitdown_client.go
│   ├── asr_interface.go
│   ├── asr_service.go
│   ├── storage_interface.go
│   └── minio_storage.go
├── repository/
│   ├── source_interface.go
│   ├── source_repository.go
│   ├── task_interface.go
│   ├── task_repository.go
│   ├── audio_preview_interface.go
│   └── audio_preview_repository.go
└── model/
    ├── entity/
    │   ├── source.go
    │   ├── import_task.go
    │   └── audio_preview.go
    └── dto/
        ├── request/import.go
        └── response/source.go
```

#### 核心接口定义

```go
// SourceService 资料来源服务
type SourceService interface {
    List(userID, notebookID uint, keyword string, page, size int) ([]*entity.Source, int64, error)
    GetByID(id uint) (*entity.Source, error)
    Rename(id uint, name string) error
    Delete(id uint) error
    BatchDelete(ids []uint) error
    GetContent(id uint) (string, error)
    GetOriginalContent(id uint) (content string, contentType string, err error)
}

// ImporterService 导入服务
type ImporterService interface {
    // 文件导入（直接）
    ImportFile(userID, notebookID uint, file *multipart.FileHeader) (*entity.Source, error)
    // 音频导入（预览+确认两步）
    PreviewAudio(userID, notebookID uint, file *multipart.FileHeader) (*entity.AudioPreview, error)
    ConfirmAudio(userID uint, previewID string, editedContent *string) (*entity.Source, error)
    // 批量导入
    ImportSearchResults(userID, notebookID uint, urls []string) (*entity.ImportTask, error)
    GetImportTask(taskID uint) (*entity.ImportTask, error)
}

// MarkitdownClient 文件解析客户端
type MarkitdownClient interface {
    Convert(filePath string) (string, error)
    ConvertFromURL(url string) (string, error)
}

// ASRService 语音转文本服务
type ASRService interface {
    Transcribe(filePath string) (string, error)
}

// FileStorage 文件存储
type FileStorage interface {
    Upload(file *multipart.FileHeader) (string, error)
    Download(filePath string) ([]byte, error)
    Delete(filePath string) error
}

// EmbeddingService 向量化服务（外部模块实现，本模块只调用）
type EmbeddingService interface {
    Vectorize(sourceID uint, content string) error
}
```

#### 类图

```
┌─────────────────────────────────────────────────────────┐
│                    ImporterService (接口)                 │
├─────────────────────────────────────────────────────────┤
│ + ImportFile(userID, nbID, file) (*Source, error)       │
│ + PreviewAudio(userID, nbID, file) (*AudioPreview, err) │
│ + ConfirmAudio(userID, previewID, edited) (*Source, err)│
│ + ImportSearchResults(userID, nbID, urls) (Task, error) │
│ + GetImportTask(taskID) (*ImportTask, error)            │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                importerService (实现)                     │
├─────────────────────────────────────────────────────────┤
│ - markitdown    MarkitdownClient                        │
│ - asr           ASRService                              │
│ - storage       FileStorage                             │
│ - sourceRepo    SourceRepository                        │
│ - taskRepo      ImportTaskRepository                    │
│ - previewRepo   AudioPreviewRepository                  │
│ - embedding     EmbeddingService                        │
├─────────────────────────────────────────────────────────┤
│ + ImportFile()                                          │
│ + PreviewAudio()                                        │
│ + ConfirmAudio()                                        │
│ + ImportSearchResults()                                 │
│ - saveAndVectorize(source) error                        │
└─────────────────────────────────────────────────────────┘
```

#### 文件导入时序图

```
User        API         ImporterService    MarkitdownClient    FileStorage    SourceRepo    EmbeddingService
 │           │               │                  │                 │              │              │
 │──上传文件─→│               │                  │                 │              │              │
 │           │──ImportFile()─→│                  │                 │              │              │
 │           │               │──Upload()────────→│                 │              │              │
 │           │               │←─filePath─────────│                 │              │              │
 │           │               │──Convert()───────→│                 │              │              │
 │           │               │←─markdown─────────│                 │              │              │
 │           │               │──Create(source)────────────────────→│              │              │
 │           │               │←─source────────────────────────────│              │              │
 │           │               │──Vectorize()──────────────────────────────────────→│              │
 │           │               │←─ok───────────────────────────────────────────────│              │
 │           │←─source───────│                  │                 │              │              │
 │←─结果─────│               │                  │                 │              │              │
```

#### 音频导入时序图（含预览）

```
User        API          ImporterService     ASRService    PreviewRepo    FileStorage
 │           │                │                 │              │              │
 │──上传音频─→│                │                 │              │              │
 │           │──PreviewAudio()→                │              │              │
 │           │                │──Upload()──────────────────────→│              │
 │           │                │←─filePath──────────────────────│              │
 │           │                │──Transcribe()──→│              │              │
 │           │                │←─text──────────│              │              │
 │           │                │──SavePreview()────────────────→│              │
 │           │                │←─preview──────────────────────│              │
 │           │←─preview───────│                │              │              │
 │           │                │                 │              │              │
 │──查看预览─→│                │                 │              │              │
 │           │                │                 │              │              │
 │──编辑内容─→│                │                 │              │              │
 │           │                │                 │              │              │
 │──确认导入─→│                │                 │              │              │
 │           │──ConfirmAudio()→                │              │              │
 │           │                │──GetPreview()──────────────────→│              │
 │           │                │←─preview──────────────────────│              │
 │           │                │──Convert(edited)───→(MarkItDown)│              │
 │           │                │←─markdown──────────│           │              │
 │           │                │──Create(source)────→(SourceRepo)│              │
 │           │                │──Vectorize()───────→(Embedding) │              │
 │           │                │──DeletePreview()───────────────→│              │
 │           │←─source────────│                │              │              │
```

#### 搜索结果批量导入时序图

```
User        API         ImporterService    MarkitdownClient    TaskRepo    SourceRepo    EmbeddingService
 │           │               │                  │                │             │              │
 │──批量导入─→│               │                  │                │             │              │
 │           │──ImportSearch()→                 │                │             │              │
 │           │               │──CreateTask()────────────────────→│             │              │
 │           │               │←─task────────────────────────────│             │              │
 │           │←─task─────────│                  │                │             │              │
 │           │               │                  │                │             │              │
 │           │ (async)       │──loop urls──┐    │                │             │              │
 │           │               │             │    │                │             │              │
 │           │               │←────────────┘    │                │             │              │
 │           │               │──Convert(url)───→│                │             │              │
 │           │               │←─markdown────────│                │             │              │
 │           │               │──Create(source)──────────────────→│             │              │
 │           │               │──Vectorize()────────────────────────────────────→│              │
 │           │               │──UpdateTask(count)───────────────→│             │              │
```

### 4.2 搜索 Agent 模块（SearchAgent）

#### 目录结构

```
internal/
├── api/v1/search/
│   ├── controller.go
│   └── routes.go
├── agent/search/
│   ├── agent.go           # Eino Agent 定义
│   ├── tools.go           # Agent 工具定义
│   └── prompts.go         # 提示词模板
├── service/
│   ├── search_agent_interface.go
│   └── search_agent_service.go
└── service/external/
    ├── search_engine_interface.go
    ├── duckduckgo_engine.go
    └── custom_engine.go
```

#### 核心接口定义

```go
// SearchAgentService 搜索Agent服务
type SearchAgentService interface {
    Search(userID, notebookID uint, query string) (*SearchResponse, error)
    ImportFromURL(userID, notebookID uint, url string) (*entity.Source, error)
}

// SearchEngine 搜索引擎接口
type SearchEngine interface {
    Search(query string, limit int) ([]SearchResultItem, error)
    Name() string
}

// ConfigService 配置服务（获取用户/系统配置，路由降级）
type ConfigService interface {
    GetSearchEngine(userID uint) (SearchEngine, error)
    GetASRService(userID uint) (ASRService, error)
    GetEmbeddingService(userID uint) (EmbeddingService, error)
}
```

#### 类图

```
┌─────────────────────────────────────────────────────────┐
│                 SearchAgentService (接口)                  │
├─────────────────────────────────────────────────────────┤
│ + Search(userID, nbID, query) (*SearchResponse, error)  │
│ + ImportFromURL(userID, nbID, url) (*Source, error)     │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│              searchAgentService (实现)                    │
├─────────────────────────────────────────────────────────┤
│ - einoAgent    *eino.Agent                               │
│ - config       ConfigService                            │
│ - importer     ImporterService                          │
├─────────────────────────────────────────────────────────┤
│ + Search()                                              │
│ + ImportFromURL()                                       │
│ - resolveSearchEngine(userID) SearchEngine              │
└─────────────────────────────────────────────────────────┘

┌──────────────────────────────┐
│    SearchEngine (接口)        │
├──────────────────────────────┤
│ + Search(query, limit)       │
│   ([]SearchResultItem, error)│
│ + Name() string              │
└──────────────────────────────┘
          ▲               ▲
          │               │
┌─────────────────┐ ┌─────────────────┐
│ DuckDuckGoEngine│ │ CustomEngine    │
├─────────────────┤ ├─────────────────┤
│ + Search()      │ │ + Search()      │
│ + Name()        │ │ + Name()        │
└─────────────────┘ └─────────────────┘
```

#### Eino Agent 设计

```go
// agent/search/agent.go

// NewSearchAgent 创建搜索Agent
func NewSearchAgent(config ConfigService, importer ImporterService) *eino.Agent {
    agent := eino.NewAgent(
        eino.WithName("search_agent"),
        eino.WithDescription("网络搜索Agent，理解用户搜索意图并调用搜索引擎"),
        eino.WithTools(
            NewSearchTool(config),
            NewURLImportTool(importer),
        ),
        eino.WithSystemPrompt(SearchSystemPrompt),
    )
    return agent
}

// agent/search/tools.go

// SearchTool 搜索工具
type SearchTool struct {
    config ConfigService
}

func (t *SearchTool) Name() string { return "web_search" }
func (t *SearchTool) Description() string {
    return "搜索网络内容，输入搜索关键词，返回搜索结果列表"
}
func (t *SearchTool) Parameters() map[string]any {
    return map[string]any{
        "query": map[string]any{
            "type":        "string",
            "description": "搜索关键词",
        },
    }
}

// URLImportTool URL导入工具
type URLImportTool struct {
    importer ImporterService
}

func (t *URLImportTool) Name() string { return "import_url" }
func (t *URLImportTool) Description() string {
    return "导入指定URL的网页内容，输入URL地址，返回导入结果"
}
```

#### 搜索 Agent 时序图

```
User        API         SearchAgentService    EinoAgent    SearchEngine    ImporterService
 │           │               │                  │              │               │
 │──搜索请求─→│               │                  │              │               │
 │           │──Search()─────→│                  │              │               │
 │           │               │──resolveEngine()──│              │               │
 │           │               │←─engine──────────│              │               │
 │           │               │──理解意图+调用───→│              │               │
 │           │               │                  │──Search()────→│               │
 │           │               │                  │←─results─────│               │
 │           │               │←─parsed results──│              │               │
 │           │←─results──────│                  │              │               │
 │           │               │                  │              │               │
 │──选择导入─→│               │                  │              │               │
 │           │──ImportURLs()→│                  │              │               │
 │           │               │──ImportSearchResults()──────────→│               │
 │           │←─task─────────│                  │              │               │
```

### 4.3 有道云笔记 Agent 模块（YoudaoAgent）

#### 目录结构

```
internal/
├── api/v1/youdao/
│   ├── controller.go
│   └── routes.go
├── agent/youdao/
│   ├── agent.go           # Eino Agent 定义
│   ├── tools.go           # Agent 工具定义
│   └── prompts.go         # 提示词模板
├── service/
│   ├── youdao_agent_interface.go
│   └── youdao_agent_service.go
└── repository/
    ├── youdao_binding_interface.go
    └── youdao_binding_repository.go
```

#### 核心接口定义

```go
// YoudaoAgentService 有道云笔记Agent服务
type YoudaoAgentService interface {
    Bind(userID uint, apiKey string) error
    Unbind(userID uint) error
    GetStatus(userID uint) (*BindingStatus, error)
    ListNotes(userID uint, page, size int, keyword string) (*NoteList, error)
    PreviewNote(userID uint, noteID string) (*NoteContent, error)
    ImportNotes(userID, notebookID uint, noteIDs []string) (*entity.ImportTask, error)
}

// YoudaoNoteSkill 有道云笔记技能（外部依赖）
type YoudaoNoteSkill interface {
    ListNotes(apiKey string, page, size int, keyword string) (*NoteList, error)
    GetNoteContent(apiKey string, noteID string) (string, error)
}

// YoudaoNoteCli 有道云笔记CLI（外部依赖）
type YoudaoNoteCli interface {
    ExportNote(apiKey string, noteID string) (string, error)
}
```

#### 类图

```
┌─────────────────────────────────────────────────────────┐
│                YoudaoAgentService (接口)                   │
├─────────────────────────────────────────────────────────┤
│ + Bind(userID, apiKey) error                            │
│ + Unbind(userID) error                                  │
│ + GetStatus(userID) (*BindingStatus, error)             │
│ + ListNotes(userID, page, keyword) (*NoteList, error)   │
│ + PreviewNote(userID, noteID) (*NoteContent, error)     │
│ + ImportNotes(userID, nbID, noteIDs) (*ImportTask, err) │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│              youdaoAgentService (实现)                    │
├─────────────────────────────────────────────────────────┤
│ - einoAgent     *eino.Agent                              │
│ - youdaoSkill   YoudaoNoteSkill                         │
│ - youdaoCli     YoudaoNoteCli                           │
│ - bindingRepo   YoudaoBindingRepository                 │
│ - importer      ImporterService                         │
│ - encrypt       EncryptService                          │
├─────────────────────────────────────────────────────────┤
│ + Bind()                                                │
│ + Unbind()                                              │
│ + ListNotes()                                           │
│ + ImportNotes()                                         │
│ - getBinding(userID) (*YoudaoBinding, error)            │
│ - validateAPIKey(apiKey) error                          │
└─────────────────────────────────────────────────────────┘
```

#### 有道笔记导入时序图

```
User        API         YoudaoAgentService    YoudaoNoteSkill    BindingRepo    ImporterService
 │           │               │                    │                 │              │
 │──绑定请求─→│               │                    │                 │              │
 │           │──Bind()───────→│                    │                 │              │
 │           │               │──validateAPIKey()──→│                 │              │
 │           │               │←─ok────────────────│                 │              │
 │           │               │──encrypt+save()──────────────────────→│              │
 │           │←─ok───────────│                    │                 │              │
 │           │               │                    │                 │              │
 │──浏览笔记─→│               │                    │                 │              │
 │           │──ListNotes()──→│                    │                 │              │
 │           │               │──getBinding()───────────────────────→│              │
 │           │               │←─binding────────────────────────────│              │
 │           │               │──ListNotes(apiKey)─→│                 │              │
 │           │               │←─notes─────────────│                 │              │
 │           │←─notes────────│                    │                 │              │
 │           │               │                    │                 │              │
 │──批量导入─→│               │                    │                 │              │
 │           │──ImportNotes()→│                    │                 │              │
 │           │               │──CreateTask()────────────────────────→│              │
 │           │               │ (async loop)       │                 │              │
 │           │               │──GetNoteContent()──→│                 │              │
 │           │               │←─markdown──────────│                 │              │
 │           │               │──ImportSearchResults()───────────────→│              │
 │           │←─task─────────│                    │                 │              │
```

### 4.4 后台管理模块（Admin）

#### 目录结构

```
internal/
├── api/v1/admin/
│   ├── controller.go
│   └── routes.go
├── service/
│   ├── admin_interface.go
│   ├── admin_service.go
│   └── config_service.go
└── repository/
    ├── sys_config_interface.go
    ├── sys_config_repository.go
    ├── user_search_config_interface.go
    ├── user_search_config_repository.go
    ├── user_asr_config_interface.go
    ├── user_asr_config_repository.go
    ├── user_embedding_config_interface.go
    └── user_embedding_config_repository.go
```

#### 核心接口定义

```go
// AdminService 后台管理服务
type AdminService interface {
    // 用户管理
    ListUsers(page, size int, keyword string) ([]*entity.User, int64, error)
    UpdateUserStatus(userID uint, enabled bool) error
    // 系统配置管理（动态，不硬编码配置项）
    GetConfigs(group string) ([]*entity.SysConfig, error)
    UpdateConfig(group, key string, value json.RawMessage, enabled bool) error
    AddConfig(group, key string, value json.RawMessage, description string) error
    GetConfigStatus() ([]ConfigStatusGroup, error)
}

// ConfigService 配置路由服务
type ConfigService interface {
    // 获取服务配置（优先用户配置，降级到系统配置）
    GetSearchEngine(userID uint) (SearchEngine, error)
    GetASRService(userID uint) (ASRService, error)
    GetEmbeddingService(userID uint) (EmbeddingService, error)
}
```

#### 类图

```
┌─────────────────────────────────────────────────────────┐
│                   AdminService (接口)                     │
├─────────────────────────────────────────────────────────┤
│ + ListUsers(page, keyword) (*UserList, error)           │
│ + UpdateUserStatus(userID, status) error                │
│ + GetConfigs(group) ([]*SysConfig, error)               │
│ + UpdateConfig(group, key, value, enabled) error        │
│ + AddConfig(group, key, value, description) error       │
│ + GetConfigStatus() ([]ConfigStatusGroup, error)        │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                 adminService (实现)                       │
├─────────────────────────────────────────────────────────┤
│ - userRepo      UserRepository                          │
│ - configRepo    SysConfigRepository                     │
├─────────────────────────────────────────────────────────┤
│ + ListUsers()                                           │
│ + UpdateUserStatus()                                    │
│ + GetConfigs()                                          │
│ + UpdateConfig()                                        │
│ + AddConfig()                                           │
│ + GetConfigStatus()                                     │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│                  ConfigService (接口)                     │
├─────────────────────────────────────────────────────────┤
│ + GetSearchEngine(userID) (SearchEngine, error)         │
│ + GetASRService(userID) (ASRService, error)             │
│ + GetEmbeddingService(userID) (EmbeddingService, error) │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                 configService (实现)                      │
├─────────────────────────────────────────────────────────┤
│ - sysConfigRepo       SysConfigRepository               │
│ - userSearchRepo      UserSearchConfigRepository        │
│ - userASRRepo         UserASRConfigRepository           │
│ - userEmbeddingRepo   UserEmbeddingConfigRepository     │
├─────────────────────────────────────────────────────────┤
│ + GetSearchEngine()                                     │
│ + GetASRService()                                       │
│ + GetEmbeddingService()                                 │
│ - resolveUserConfig(userID, serviceType) (Config, bool) │
│ - loadBuiltinConfig(serviceType) Config                 │
└─────────────────────────────────────────────────────────┘
```

### 4.5 服务路由降级流程

```
用户请求某个服务（如搜索）
        │
        ▼
  查用户自定义配置（enabled=true, 按priority排序）
        │
   ┌────┴────┐
   │ 有配置？ │
   └────┬────┘
    yes │        no
    ▼   │         ▼
  使用用户配置   查内置配置（sys_config）
    │              │
    ▼              ▼
  调用服务       使用内置配置
    │              │
    ▼              ▼
  调用失败？     调用服务
    │ yes         │
    ▼              ▼
  尝试下一个配置  返回结果
    │
    ▼
  全部失败 → 返回错误
```

**路由降级伪代码：**

```go
func (s *configService) GetSearchEngine(userID uint) (SearchEngine, error) {
    // 1. 查用户配置（按priority排序）
    userConfigs, _ := s.userSearchRepo.FindByUser(userID)
    for _, cfg := range userConfigs {
        if !cfg.Enabled {
            continue
        }
        engine, err := s.createSearchEngine(cfg)
        if err == nil {
            return engine, nil
        }
    }

    // 2. 降级到内置配置
    builtins, _ := s.sysConfigRepo.FindByGroup("search")
    for _, builtin := range builtins {
        if builtin.Enabled {
            engine, err := s.createBuiltinSearchEngine(builtin)
            if err == nil {
                return engine, nil
            }
        }
    }

    // 3. 使用 DuckDuckGo 兜底
    return NewDuckDuckGoEngine(), nil
}
```

---

## 5. 错误处理与降级策略

### 5.1 文件导入错误处理

| 场景 | 处理方式 |
|------|----------|
| 不支持的文件格式 | 返回 40001 错误，提示支持的格式列表 |
| 文件大小超限 | 返回 40002 错误，提示大小限制 |
| MarkItDown 解析失败 | 返回 40003 错误，保留原始文件，允许重试 |
| Embedding 服务不可用 | Source 保存成功，vectorized=false，后续可重试 |

### 5.2 音频导入错误处理

| 场景 | 处理方式 |
|------|----------|
| Whisper 服务不可用 | 返回 40005 错误，提示ASR服务未配置 |
| 转写质量过低 | 仍然返回预览，用户可编辑修正 |
| 预览过期 | 返回 40009 错误，提示重新上传 |

### 5.3 搜索导入错误处理

| 场景 | 处理方式 |
|------|----------|
| 搜索API配额耗尽 | 返回 40006 错误，提示配额用尽 |
| 网页抓取失败 | 跳过该URL，记录到 task.error_detail，不影响其他URL |
| 全部URL失败 | task.status = "partial_failed" |

### 5.4 有道云笔记错误处理

| 场景 | 处理方式 |
|------|----------|
| API Key 无效 | 返回 40007 错误，提示重新输入 |
| 网络超时 | 自动重试 3 次，失败后记录日志 |
| 笔记内容为空 | 跳过该笔记，不影响其他笔记导入 |

---

## 6. 安全设计

### 6.1 API Key 加密存储

- 有道云笔记 API Key、用户配置的 API Key 均使用 AES-256 加密后存储
- 加密密钥从环境变量读取，不硬编码
- 日志中不打印 API Key

### 6.2 权限控制

- 所有接口需要 JWT 认证（已有中间件）
- 后台管理接口需要 admin 角色
- 用户只能访问自己的数据

### 6.3 输入校验

- 文件类型白名单校验
- 文件大小限制校验
- URL 格式校验
- SQL 注入防护（GORM 默认参数化查询）

---

## 7. 测试策略

### 7.1 单元测试

- 每个 Service 实现编写单元测试
- Mock 外部依赖（MarkItDown、ASR、搜索引擎）
- 测试覆盖正常流程和异常流程

### 7.2 集成测试

- API 接口集成测试
- 数据库操作集成测试
- Agent 工具调用集成测试

### 7.3 测试用例

| 模块 | 测试场景 |
|------|----------|
| 文件导入 | 支持的格式、不支持的格式、大小超限、解析失败 |
| 音频导入 | 正常转写、预览、编辑后确认、预览过期、ASR不可用 |
| 搜索导入 | 关键词搜索、URL直接导入、批量导入、部分失败 |
| 有道导入 | 绑定、解绑、浏览、批量导入 |
| 配置路由 | 用户配置优先、降级到内置、全部失败 |
