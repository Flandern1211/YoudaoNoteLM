#!/bin/bash
# SSH 密钥配置脚本
# 作用：帮助配置 SSH 密钥到服务器
# 使用方法：bash setup-ssh.sh

set -e  # 遇到错误立即退出

echo "🔑 开始配置 SSH 密钥..."

# 配置变量
SERVER_IP="60.205.184.232"
SERVER_USER="root"
SSH_KEY="$HOME/.ssh/github_actions.pub"

# 检查公钥是否存在
if [ ! -f "$SSH_KEY" ]; then
    echo "❌ 错误：找不到公钥文件 $SSH_KEY"
    echo "请先运行：ssh-keygen -t ed25519 -C 'github-actions' -f ~/.ssh/github_actions"
    exit 1
fi

echo "📋 公钥内容："
cat "$SSH_KEY"
echo ""

# 复制公钥到服务器
echo "📤 复制公钥到服务器..."
ssh-copy-id -i "$SSH_KEY" "$SERVER_USER@$SERVER_IP"

# 测试连接
echo "🔗 测试 SSH 连接..."
ssh -i ~/.ssh/github_actions "$SERVER_USER@$SERVER_IP" "echo '✅ SSH 连接成功！'"

# 创建部署目录
echo "📁 创建部署目录..."
ssh -i ~/.ssh/github_actions "$SERVER_USER@$SERVER_IP" "mkdir -p /home/flandern/youdaonotelm/{configs,logs}"

# 上传初始化脚本
echo "📤 上传初始化脚本..."
scp -i ~/.ssh/github_actions .github/scripts/server-setup.sh "$SERVER_USER@$SERVER_IP:/home/flandern/"

# 执行初始化脚本
echo "🚀 执行服务器初始化脚本..."
ssh -i ~/.ssh/github_actions "$SERVER_USER@$SERVER_IP" "cd /home/flandern && bash server-setup.sh"

echo ""
echo "🎉 SSH 配置完成！"
echo ""
echo "📋 下一步："
echo "1. 将私钥内容添加到 GitHub Secrets："
echo "   - SSH_HOST: $SERVER_IP"
echo "   - SSH_USERNAME: $SERVER_USER"
echo "   - SSH_PRIVATE_KEY: 粘贴私钥内容"
echo ""
echo "2. 上传配置文件："
echo "   scp configs/config.yaml $SERVER_USER@$SERVER_IP:/home/flandern/youdaonotelm/configs/"
echo ""
echo "3. 开始使用 CI/CD！"
echo ""
echo "✅ 配置完成！"
