#!/bin/bash
# 服务器初始化脚本
# 作用：在 CentOS 7 服务器上初始化部署环境
# 使用方法：在服务器上执行 bash server-setup.sh

set -e  # 遇到错误立即退出

echo "🚀 开始初始化服务器环境..."

# 配置变量
DEPLOY_DIR="/home/flandern/youdaonotelm"
LOG_DIR="$DEPLOY_DIR/logs"
CONFIG_DIR="$DEPLOY_DIR/configs"

# 1. 创建必要的目录
echo "📁 创建目录结构..."
mkdir -p "$DEPLOY_DIR"
mkdir -p "$LOG_DIR"
mkdir -p "$CONFIG_DIR"

# 2. 安装 Supervisor（如果未安装）
echo "📦 检查并安装 Supervisor..."
if ! command -v supervisord &> /dev/null; then
    echo "安装 Supervisor..."
    yum install -y epel-release
    yum install -y supervisor
    systemctl enable supervisord
    systemctl start supervisord
    echo "✅ Supervisor 安装完成"
else
    echo "✅ Supervisor 已安装"
fi

# 3. 配置 Supervisor
echo "⚙️ 配置 Supervisor..."
SUPERVISOR_CONFIG="/etc/supervisor/conf.d/youdaonotelm-server.conf"

# 备份现有配置
if [ -f "$SUPERVISOR_CONFIG" ]; then
    cp "$SUPERVISOR_CONFIG" "$SUPERVISOR_CONFIG.backup.$(date +%Y%m%d_%H%M%S)"
fi

# 创建新的 Supervisor 配置
cat > "$SUPERVISOR_CONFIG" << 'EOF'
[program:youdaonotelm-server]
command=/home/flandern/youdaonotelm/youdaonotelm-server
directory=/home/flandern/youdaonotelm
user=root
autostart=true
autorestart=true
startsecs=10
startretries=3
redirect_stderr=true
stdout_logfile=/home/flandern/youdaonotelm/logs/server.log
stdout_logfile_maxbytes=50MB
stdout_logfile_backups=10
environment=GIN_MODE="release",GO_ENV="production"
priority=999
killasgroup=true
stopasgroup=true
stopsignal=SIGTERM
stopwaitsecs=30
EOF

echo "✅ Supervisor 配置完成"

# 4. 重新加载 Supervisor 配置
echo "🔄 重新加载 Supervisor 配置..."
supervisorctl reread
supervisorctl update

# 5. 设置目录权限
echo "🔐 设置目录权限..."
chmod -R 755 "$DEPLOY_DIR"
chmod -R 755 "$LOG_DIR"
chmod -R 755 "$CONFIG_DIR"

# 6. 配置防火墙（开放 8080 端口）
echo "🔥 配置防火墙..."
if systemctl is-active --quiet firewalld; then
    firewall-cmd --permanent --add-port=8080/tcp
    firewall-cmd --reload
    echo "✅ 防火墙配置完成（开放 8080 端口）"
else
    echo "⚠️ firewalld 未运行，请手动配置防火墙"
fi

# 7. 创建配置文件模板
echo "📝 创建配置文件模板..."
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    cat > "$CONFIG_DIR/config.yaml.example" << 'EOF'
# YoudaoNoteLM 配置文件示例
# 请复制此文件为 config.yaml 并修改配置

server:
  port: 8080
  mode: release

database:
  host: localhost
  port: 3306
  user: root
  password: your_password
  dbname: youdaonotelm

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0

jwt:
  secret: your_jwt_secret_key
  expire: 24h
EOF
    echo "✅ 配置文件模板创建完成"
fi

# 8. 检查 Go 环境（可选）
echo "🔍 检查 Go 环境..."
if command -v go &> /dev/null; then
    go_version=$(go version)
    echo "✅ Go 已安装: $go_version"
else
    echo "⚠️ Go 未安装（CI/CD 会在 GitHub 上构建，服务器不需要 Go）"
fi

# 9. 显示初始化结果
echo ""
echo "🎉 服务器初始化完成！"
echo ""
echo "📁 目录结构："
echo "  $DEPLOY_DIR/"
echo "  ├── youdaonotelm-server    # 应用程序"
echo "  ├── configs/"
echo "  │   └── config.yaml        # 配置文件"
echo "  └── logs/"
echo "      └── server.log         # 应用日志"
echo ""
echo "📋 后续步骤："
echo "1. 上传配置文件："
echo "   scp configs/config.yaml root@60.205.184.232:$CONFIG_DIR/"
echo ""
echo "2. 上传应用（如果手动部署）："
echo "   scp youdaonotelm-server root@60.205.184.232:$DEPLOY_DIR/"
echo ""
echo "3. 启动服务："
echo "   supervisorctl start youdaonotelm-server"
echo ""
echo "4. 查看服务状态："
echo "   supervisorctl status youdaonotelm-server"
echo ""
echo "5. 查看日志："
echo "   tail -f $LOG_DIR/server.log"
echo ""
echo "✅ 初始化完成！"
