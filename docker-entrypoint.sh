#!/bin/sh

# 切换到 /app 目录
cd /app

# 启动后端服务（后台运行）
/app/server &

# 启动 Nginx（前台运行）
nginx -g "daemon off;"
