#!/bin/bash
# Supervisor 配置脚本
# 作用：在 CentOS 7 服务器上配置 Supervisor
# 使用方法：bash setup-supervisor.sh

set -e  # 遇到错误立即退出

echo "🔧 开始配置 Supervisor..."

# 配置变量
DEPLOY_DIR="/home/flandern/youdaonotelm"
LOG_DIR="$DEPLOY_DIR/logs"
CONFIG_DIR="$DEPLOY_DIR/configs"
SUPERVISOR_CONFIG="/etc/supervisor/conf.d/youdaonotelm-server.conf"

# 1. 创建必要的目录
echo "📁 创建目录结构..."
mkdir -p "$DEPLOY_DIR"
mkdir -p "$LOG_DIR"
mkdir -p "$CONFIG_DIR"

# 2. 检查 Supervisor 是否安装
echo "📦 检查 Supervisor..."
if ! command -v supervisord &> /dev/null; then
    echo "❌ Supervisor 未安装，请先安装 Supervisor"
    echo "运行：sudo yum install -y supervisor"
    exit 1
fi

# 3. 备份现有配置
echo "💾 备份现有配置..."
if [ -f "$SUPERVISOR_CONFIG" ]; then
    cp "$SUPERVISOR_CONFIG" "$SUPERVISOR_CONFIG.backup.$(date +%Y%m%d_%H%M%S)"
    echo "✅ 已备份到 $SUPERVISOR_CONFIG.backup.*"
fi

# 4. 创建 Supervisor 配置
echo "📝 创建 Supervisor 配置..."
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

echo "✅ Supervisor 配置创建完成"

# 5. 重新加载 Supervisor 配置
echo "🔄 重新加载 Supervisor 配置..."
supervisorctl reread
supervisorctl update

# 6. 设置目录权限
echo "🔐 设置目录权限..."
chmod -R 755 "$DEPLOY_DIR"
chmod -R 755 "$LOG_DIR"
chmod -R 755 "$CONFIG_DIR"

# 7. 检查配置文件
echo "📋 检查配置文件..."
if [ -f "$SUPERVISOR_CONFIG" ]; then
    echo "✅ 配置文件存在：$SUPERVISOR_CONFIG"
    echo ""
    echo "配置内容："
    cat "$SUPERVISOR_CONFIG"
else
    echo "❌ 配置文件创建失败"
    exit 1
fi

# 8. 显示配置结果
echo ""
echo "🎉 Supervisor 配置完成！"
echo ""
echo "📁 目录结构："
echo "  $DEPLOY_DIR/"
echo "  ├── youdaonotelm-server    # 应用程序"
echo "  ├── configs/"
echo "  │   └── config.yaml        # 配置文件"
echo "  └── logs/"
echo "      └── server.log         # 应用日志"
echo ""
echo "📋 常用命令："
echo "  # 查看服务状态"
echo "  sudo supervisorctl status youdaonotelm-server"
echo ""
echo "  # 启动服务"
echo "  sudo supervisorctl start youdaonotelm-server"
echo ""
echo "  # 停止服务"
echo "  sudo supervisorctl stop youdaonotelm-server"
echo ""
echo "  # 重启服务"
echo "  sudo supervisorctl restart youdaonotelm-server"
echo ""
echo "  # 查看日志"
echo "  tail -f $LOG_DIR/server.log"
echo ""
echo "  # 查看 Supervisor 日志"
echo "  sudo supervisorctl tail youdaonotelm-server stderr"
echo ""
echo "✅ 配置完成！"
