#!/bin/bash
# LiteChat 部署脚本（兼容旧版 docker-compose）
set -e

echo ">>> 拉取最新代码..."
git pull

echo ">>> 停止并删除旧容器..."
docker-compose down --remove-orphans 2>/dev/null || true
docker rm -f litechat 2>/dev/null || true

echo ">>> 构建镜像..."
docker-compose build

echo ">>> 启动容器..."
docker-compose up -d

echo ">>> 部署完成！"
docker ps | grep litechat
