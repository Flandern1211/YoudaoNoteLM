# YoudaoNoteLM API cURL 测试示例

## 基础配置

```bash
# 基础URL
BASE_URL="http://localhost:8080/api/v1"

# 测试用户
EMAIL="test@example.com"
PASSWORD="Test123456"
```

---

## 1. 认证模块

### 1.1 获取验证码

```bash
curl -X GET "${BASE_URL}/auth/captcha" \
  -H "Content-Type: application/json" | jq
```

**预期响应:**
```json
{
  "code": 0,
  "message": "成功",
  "data": {
    "captcha_id": "uuid-string",
    "background": "base64...",
    "slider": "base64...",
    "slider_size": 50,
    "bg_width": 300,
    "slider_start_x": 10
  }
}
```

### 1.2 发送验证码

```bash
curl -X POST "${BASE_URL}/auth/send-code" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "type": "register"
  }' | jq
```

### 1.3 用户注册

```bash
curl -X POST "${BASE_URL}/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123456",
    "confirm_password": "Test123456",
    "code": "123456"
  }' | jq
```

### 1.4 用户登录

```bash
# 先获取验证码
CAPTCHA_RESPONSE=$(curl -s -X GET "${BASE_URL}/auth/captcha")
CAPTCHA_ID=$(echo $CAPTCHA_RESPONSE | jq -r '.data.captcha_id')

# 登录
curl -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"password\": \"${PASSWORD}\",
    \"captcha_id\": \"${CAPTCHA_ID}\",
    \"captcha_x\": 150
  }" | jq
```

**保存Token:**
```bash
# 登录后保存token
LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"password\": \"${PASSWORD}\",
    \"captcha_id\": \"${CAPTCHA_ID}\",
    \"captcha_x\": 150
  }")

ACCESS_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.access_token')
REFRESH_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.refresh_token')

echo "Access Token: ${ACCESS_TOKEN}"
echo "Refresh Token: ${REFRESH_TOKEN}"
```

### 1.5 刷新Token

```bash
curl -X POST "${BASE_URL}/auth/refresh" \
  -H "Content-Type: application/json" \
  -d "{
    \"refresh_token\": \"${REFRESH_TOKEN}\"
  }" | jq
```

### 1.6 用户登出

```bash
curl -X POST "${BASE_URL}/auth/logout" \
  -H "Content-Type: application/json" \
  -d "{
    \"access_token\": \"${ACCESS_TOKEN}\",
    \"refresh_token\": \"${REFRESH_TOKEN}\"
  }" | jq
```

---

## 2. 用户模块

### 2.1 获取用户信息

```bash
curl -X GET "${BASE_URL}/user/profile" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 2.2 更新用户信息

```bash
curl -X PUT "${BASE_URL}/user/profile" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "nickname": "测试用户",
    "avatar": "https://example.com/avatar.jpg"
  }' | jq
```

### 2.3 修改用户名

```bash
curl -X PUT "${BASE_URL}/user/username" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newusername"
  }' | jq
```

### 2.4 上传头像

```bash
curl -X POST "${BASE_URL}/user/avatar" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -F "avatar=@/path/to/avatar.jpg" | jq
```

### 2.5 修改密码

```bash
curl -X POST "${BASE_URL}/user/password" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "old_password": "Test123456",
    "new_password": "NewTest123456"
  }' | jq
```

### 2.6 注销账号

```bash
# 先发送验证码
curl -X POST "${BASE_URL}/auth/send-code" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"type\": \"delete_account\"
  }"

# 注销账号
curl -X DELETE "${BASE_URL}/user/account" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "password": "Test123456",
    "code": "123456"
  }' | jq
```

### 2.7 用户列表

```bash
curl -X GET "${BASE_URL}/user/list?page=1&size=20" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

---

## 3. 笔记本模块

### 3.1 创建笔记本

```bash
curl -X POST "${BASE_URL}/notebooks" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的笔记本"
  }' | jq
```

**保存笔记本ID:**
```bash
NB_RESPONSE=$(curl -s -X POST "${BASE_URL}/notebooks" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name": "测试笔记本"}')

NOTEBOOK_ID=$(echo $NB_RESPONSE | jq -r '.data.id')
echo "Notebook ID: ${NOTEBOOK_ID}"
```

### 3.2 笔记本列表

```bash
curl -X GET "${BASE_URL}/notebooks" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 3.3 重命名笔记本

```bash
curl -X PUT "${BASE_URL}/notebooks/${NOTEBOOK_ID}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "新笔记本名称"
  }' | jq
```

### 3.4 删除笔记本

```bash
curl -X DELETE "${BASE_URL}/notebooks/${NOTEBOOK_ID}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

---

## 4. 资料来源模块

### 4.1 来源列表

```bash
curl -X GET "${BASE_URL}/notebooks/${NOTEBOOK_ID}/sources?page=1&size=10" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 4.2 来源详情

```bash
SOURCE_ID=1
curl -X GET "${BASE_URL}/notebooks/${NOTEBOOK_ID}/sources/${SOURCE_ID}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 4.3 重命名来源

```bash
curl -X PUT "${BASE_URL}/notebooks/${NOTEBOOK_ID}/sources/${SOURCE_ID}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "新来源名称"
  }' | jq
```

### 4.4 删除来源

```bash
curl -X DELETE "${BASE_URL}/notebooks/${NOTEBOOK_ID}/sources/${SOURCE_ID}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 4.5 批量删除

```bash
curl -X POST "${BASE_URL}/notebooks/${NOTEBOOK_ID}/sources/batch-delete" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "ids": [1, 2, 3]
  }' | jq
```

### 4.6 获取内容

```bash
curl -X GET "${BASE_URL}/notebooks/${NOTEBOOK_ID}/sources/${SOURCE_ID}/content" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 4.7 获取原格式

```bash
curl -X GET "${BASE_URL}/notebooks/${NOTEBOOK_ID}/sources/${SOURCE_ID}/original" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

---

## 5. 导入模块

### 5.1 文件导入

```bash
curl -X POST "${BASE_URL}/notebooks/${NOTEBOOK_ID}/import/file" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -F "file=@/path/to/document.pdf" | jq
```

### 5.2 音频预览

```bash
curl -X POST "${BASE_URL}/notebooks/${NOTEBOOK_ID}/import/audio/preview" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -F "file=@/path/to/audio.mp3" | jq
```

**保存预览ID:**
```bash
PREVIEW_RESPONSE=$(curl -s -X POST "${BASE_URL}/notebooks/${NOTEBOOK_ID}/import/audio/preview" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -F "file=@/path/to/audio.mp3")

PREVIEW_ID=$(echo $PREVIEW_RESPONSE | jq -r '.data.preview_id')
echo "Preview ID: ${PREVIEW_ID}"
```

### 5.3 确认音频导入

```bash
curl -X POST "${BASE_URL}/import/audio/confirm" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"preview_id\": \"${PREVIEW_ID}\",
    \"content\": \"修改后的转写文本...\",
    \"notebook_id\": ${NOTEBOOK_ID}
  }" | jq
```

### 5.4 查询任务

```bash
TASK_ID="task-uuid"
curl -X GET "${BASE_URL}/import/tasks/${TASK_ID}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

---

## 6. 后台管理模块

### 6.1 用户列表

```bash
curl -X GET "${BASE_URL}/admin/users?page=1&size=10" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 6.2 启用/禁用用户

```bash
USER_ID=1
curl -X PUT "${BASE_URL}/admin/users/${USER_ID}/status" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": false
  }' | jq
```

### 6.3 获取配置状态汇总

```bash
curl -X GET "${BASE_URL}/admin/config/status" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 6.4 获取配置组

```bash
GROUP="search"
curl -X GET "${BASE_URL}/admin/config/${GROUP}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 6.5 新增配置

```bash
GROUP="search"
curl -X POST "${BASE_URL}/admin/config/${GROUP}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "config_key": "bing",
    "config_value": {
      "api_url": "https://api.bing.com/v7.0/search",
      "api_key": "your-api-key"
    },
    "description": "Bing 搜索引擎配置"
  }' | jq
```

### 6.6 更新配置

```bash
GROUP="search"
KEY="bing"
curl -X PUT "${BASE_URL}/admin/config/${GROUP}/${KEY}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "config_value": {
      "api_url": "https://api.bing.com/v7.0/search",
      "api_key": "new-api-key"
    },
    "enabled": true
  }' | jq
```

---

## 7. 用户配置模块

### 7.1 搜索配置列表

```bash
curl -X GET "${BASE_URL}/user/config/search" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 7.2 创建搜索配置

```bash
curl -X POST "${BASE_URL}/user/config/search" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的搜索配置",
    "provider": "bing",
    "api_key": "your-api-key",
    "api_url": "https://api.bing.com/v7.0/search",
    "daily_quota": 100
  }' | jq
```

### 7.3 更新搜索配置

```bash
CONFIG_ID=1
curl -X PUT "${BASE_URL}/user/config/search/${CONFIG_ID}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "更新后的搜索配置",
    "provider": "bing",
    "api_key": "new-api-key",
    "api_url": "https://api.bing.com/v7.0/search",
    "daily_quota": 200
  }' | jq
```

### 7.4 删除搜索配置

```bash
CONFIG_ID=1
curl -X DELETE "${BASE_URL}/user/config/search/${CONFIG_ID}" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 7.5 ASR 配置列表

```bash
curl -X GET "${BASE_URL}/user/config/asr" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 7.6 创建 ASR 配置

```bash
curl -X POST "${BASE_URL}/user/config/asr" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "阿里云 ASR",
    "provider": "aliyun",
    "api_key": "your-access-key-id",
    "api_url": "https://nls-gateway.cn-shanghai.aliyuncs.com",
    "extra_config": {
      "access_key_secret": "your-access-key-secret",
      "app_key": "your-app-key"
    }
  }' | jq
```

### 7.7 Embedding 配置列表

```bash
curl -X GET "${BASE_URL}/user/config/embedding" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq
```

### 7.8 创建 Embedding 配置

```bash
curl -X POST "${BASE_URL}/user/config/embedding" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "OpenAI Embedding",
    "provider": "openai",
    "api_key": "your-api-key",
    "api_url": "https://api.openai.com/v1",
    "model": "text-embedding-3-small",
    "dimensions": 1536
  }' | jq
```

---

## 8. 搜索模块

### 6.1 智能搜索

```bash
curl -X POST "${BASE_URL}/notebooks/${NOTEBOOK_ID}/search" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "人工智能发展趋势"
  }' | jq
```

### 6.2 URL导入

```bash
curl -X POST "${BASE_URL}/notebooks/${NOTEBOOK_ID}/search/url" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/article"
  }' | jq
```

### 6.3 批量导入

```bash
curl -X POST "${BASE_URL}/notebooks/${NOTEBOOK_ID}/search/import" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "urls": [
      "https://example.com/article1",
      "https://example.com/article2"
    ]
  }' | jq
```

---

## 7. 错误场景测试

### 7.1 未认证访问

```bash
# 不带Authorization头
curl -X GET "${BASE_URL}/user/profile" | jq

# 预期响应
# {
#   "code": 401,
#   "message": "请提供认证令牌"
# }
```

### 7.2 Token过期

```bash
# 使用过期的token
EXPIRED_TOKEN="eyJ..."
curl -X GET "${BASE_URL}/user/profile" \
  -H "Authorization: Bearer ${EXPIRED_TOKEN}" | jq

# 预期响应
# {
#   "code": 1006,
#   "message": "令牌已过期"
# }
```

### 7.3 参数校验失败

```bash
# 密码过短
curl -X POST "${BASE_URL}/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "123",
    "confirm_password": "123",
    "code": "123456"
  }' | jq

# 预期响应
# {
#   "code": 400,
#   "message": "..."
# }
```

### 7.4 资源不存在

```bash
# 访问不存在的笔记本
curl -X GET "${BASE_URL}/notebooks/99999" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

# 预期响应
# {
#   "code": 3001,
#   "message": "资源不存在"
# }
```

---

## 8. 完整测试流程脚本

```bash
#!/bin/bash

# 完整测试流程
BASE_URL="http://localhost:8080/api/v1"
EMAIL="test_$(date +%s)@example.com"
PASSWORD="Test123456"

echo "=== 1. 获取验证码 ==="
CAPTCHA_RESPONSE=$(curl -s -X GET "${BASE_URL}/auth/captcha")
CAPTCHA_ID=$(echo $CAPTCHA_RESPONSE | jq -r '.data.captcha_id')
echo "Captcha ID: ${CAPTCHA_ID}"

echo -e "\n=== 2. 发送验证码 ==="
curl -s -X POST "${BASE_URL}/auth/send-code" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"type\": \"register\"
  }" | jq

echo -e "\n=== 3. 用户注册 ==="
curl -s -X POST "${BASE_URL}/auth/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"password\": \"${PASSWORD}\",
    \"confirm_password\": \"${PASSWORD}\",
    \"code\": \"123456\"
  }" | jq

echo -e "\n=== 4. 用户登录 ==="
LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"password\": \"${PASSWORD}\",
    \"captcha_id\": \"${CAPTCHA_ID}\",
    \"captcha_x\": 150
  }")

ACCESS_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.access_token')
REFRESH_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.refresh_token')
echo "Access Token: ${ACCESS_TOKEN:0:50}..."
echo "Refresh Token: ${REFRESH_TOKEN:0:50}..."

echo -e "\n=== 5. 获取用户信息 ==="
curl -s -X GET "${BASE_URL}/user/profile" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 6. 创建笔记本 ==="
NB_RESPONSE=$(curl -s -X POST "${BASE_URL}/notebooks" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name": "测试笔记本"}')
NOTEBOOK_ID=$(echo $NB_RESPONSE | jq -r '.data.id')
echo "Notebook ID: ${NOTEBOOK_ID}"

echo -e "\n=== 7. 笔记本列表 ==="
curl -s -X GET "${BASE_URL}/notebooks" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 8. 刷新Token ==="
REFRESH_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/refresh" \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"${REFRESH_TOKEN}\"}")
NEW_ACCESS_TOKEN=$(echo $REFRESH_RESPONSE | jq -r '.data.access_token')
echo "New Access Token: ${NEW_ACCESS_TOKEN:0:50}..."

echo -e "\n=== 9. 登出 ==="
curl -s -X POST "${BASE_URL}/auth/logout" \
  -H "Content-Type: application/json" \
  -d "{
    \"access_token\": \"${NEW_ACCESS_TOKEN}\",
    \"refresh_token\": \"${REFRESH_TOKEN}\"
  }" | jq

echo -e "\n=== 测试完成 ==="
```

---

## 9. 错误场景测试

### 9.1 未认证访问

```bash
curl -X GET "http://localhost:8080/api/v1/health" | jq

# 预期响应
# {
#   "status": "ok",
#   "message": "YouDaoNoteLM API is running"
# }
```

---

## 10. 完整测试流程脚本（更新版）

```bash
#!/bin/bash

# 完整测试流程（包含后台管理和用户配置）
BASE_URL="http://localhost:8080/api/v1"
EMAIL="test_$(date +%s)@example.com"
PASSWORD="Test123456"

echo "=== 1. 获取验证码 ==="
CAPTCHA_RESPONSE=$(curl -s -X GET "${BASE_URL}/auth/captcha")
CAPTCHA_ID=$(echo $CAPTCHA_RESPONSE | jq -r '.data.captcha_id')
echo "Captcha ID: ${CAPTCHA_ID}"

echo -e "\n=== 2. 发送验证码 ==="
curl -s -X POST "${BASE_URL}/auth/send-code" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"type\": \"register\"
  }" | jq

echo -e "\n=== 3. 用户注册 ==="
curl -s -X POST "${BASE_URL}/auth/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"password\": \"${PASSWORD}\",
    \"confirm_password\": \"${PASSWORD}\",
    \"code\": \"123456\"
  }" | jq

echo -e "\n=== 4. 用户登录 ==="
LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"password\": \"${PASSWORD}\",
    \"captcha_id\": \"${CAPTCHA_ID}\",
    \"captcha_x\": 150
  }")

ACCESS_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.access_token')
REFRESH_TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.data.refresh_token')
echo "Access Token: ${ACCESS_TOKEN:0:50}..."
echo "Refresh Token: ${REFRESH_TOKEN:0:50}..."

echo -e "\n=== 5. 获取用户信息 ==="
curl -s -X GET "${BASE_URL}/user/profile" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 6. 创建笔记本 ==="
NB_RESPONSE=$(curl -s -X POST "${BASE_URL}/notebooks" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name": "测试笔记本"}')
NOTEBOOK_ID=$(echo $NB_RESPONSE | jq -r '.data.id')
echo "Notebook ID: ${NOTEBOOK_ID}"

echo -e "\n=== 7. 笔记本列表 ==="
curl -s -X GET "${BASE_URL}/notebooks" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 8. 用户配置 - 搜索配置列表 ==="
curl -s -X GET "${BASE_URL}/user/config/search" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 9. 用户配置 - 创建搜索配置 ==="
curl -s -X POST "${BASE_URL}/user/config/search" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的搜索配置",
    "provider": "bing",
    "api_key": "test-api-key",
    "api_url": "https://api.bing.com/v7.0/search",
    "daily_quota": 100
  }' | jq

echo -e "\n=== 10. 用户配置 - ASR 配置列表 ==="
curl -s -X GET "${BASE_URL}/user/config/asr" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 11. 用户配置 - Embedding 配置列表 ==="
curl -s -X GET "${BASE_URL}/user/config/embedding" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 12. 后台管理 - 用户列表 ==="
curl -s -X GET "${BASE_URL}/admin/users?page=1&size=10" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 13. 后台管理 - 配置状态汇总 ==="
curl -s -X GET "${BASE_URL}/admin/config/status" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 14. 后台管理 - 获取搜索配置组 ==="
curl -s -X GET "${BASE_URL}/admin/config/search" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq

echo -e "\n=== 15. 后台管理 - 新增系统配置 ==="
curl -s -X POST "${BASE_URL}/admin/config/search" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "config_key": "duckduckgo",
    "config_value": {
      "enabled": true
    },
    "description": "DuckDuckGo 兜底搜索引擎"
  }' | jq

echo -e "\n=== 16. 刷新Token ==="
REFRESH_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/refresh" \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"${REFRESH_TOKEN}\"}")
NEW_ACCESS_TOKEN=$(echo $REFRESH_RESPONSE | jq -r '.data.access_token')
echo "New Access Token: ${NEW_ACCESS_TOKEN:0:50}..."

echo -e "\n=== 17. 登出 ==="
curl -s -X POST "${BASE_URL}/auth/logout" \
  -H "Content-Type: application/json" \
  -d "{
    \"access_token\": \"${NEW_ACCESS_TOKEN}\",
    \"refresh_token\": \"${REFRESH_TOKEN}\"
  }" | jq

echo -e "\n=== 测试完成 ==="
```
