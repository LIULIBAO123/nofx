#!/bin/bash
# NOFX Docker 快速更新脚本
# 用于更新 Meridian 设计系统后的前端部署

set -e

echo "🚀 NOFX Docker 更新脚本"
echo "========================"
echo ""

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}❌ Docker 未运行，请先启动 Docker${NC}"
    exit 1
fi

echo -e "${BLUE}📦 当前服务状态:${NC}"
docker-compose ps
echo ""

# 询问更新类型
echo -e "${YELLOW}请选择更新类型:${NC}"
echo "1) 完整更新（前端 + 后端）"
echo "2) 仅更新前端（推荐用于 Meridian 设计系统更新）"
echo "3) 仅更新后端"
echo "4) 快速重启"
read -p "请输入选项 (1-4): " choice

case $choice in
    1)
        echo -e "${BLUE}🔄 执行完整更新...${NC}"
        echo ""
        
        echo -e "${BLUE}1/5 停止服务...${NC}"
        docker-compose down
        
        echo -e "${BLUE}2/5 拉取最新代码...${NC}"
        git pull
        
        echo -e "${BLUE}3/5 重新构建镜像（不使用缓存）...${NC}"
        docker-compose build --no-cache
        
        echo -e "${BLUE}4/5 启动服务...${NC}"
        docker-compose up -d
        
        echo -e "${BLUE}5/5 等待服务就绪...${NC}"
        sleep 10
        ;;
        
    2)
        echo -e "${BLUE}🎨 执行前端更新（Meridian 设计系统）...${NC}"
        echo ""
        
        echo -e "${BLUE}1/5 停止前端服务...${NC}"
        docker-compose stop nofx-frontend
        
        echo -e "${BLUE}2/5 删除旧镜像...${NC}"
        docker rmi nofx-nofx-frontend 2>/dev/null || true
        
        echo -e "${BLUE}3/5 重新构建前端（不使用缓存）...${NC}"
        docker-compose build --no-cache nofx-frontend
        
        echo -e "${BLUE}4/5 启动前端服务...${NC}"
        docker-compose up -d nofx-frontend
        
        echo -e "${BLUE}5/5 等待服务就绪...${NC}"
        sleep 5
        ;;
        
    3)
        echo -e "${BLUE}⚙️  执行后端更新...${NC}"
        echo ""
        
        echo -e "${BLUE}1/5 停止后端服务...${NC}"
        docker-compose stop nofx
        
        echo -e "${BLUE}2/5 删除旧镜像...${NC}"
        docker rmi nofx-nofx 2>/dev/null || true
        
        echo -e "${BLUE}3/5 重新构建后端（不使用缓存）...${NC}"
        docker-compose build --no-cache nofx
        
        echo -e "${BLUE}4/5 启动后端服务...${NC}"
        docker-compose up -d nofx
        
        echo -e "${BLUE}5/5 等待服务就绪...${NC}"
        sleep 10
        ;;
        
    4)
        echo -e "${BLUE}⚡ 快速重启...${NC}"
        echo ""
        docker-compose restart
        sleep 5
        ;;
        
    *)
        echo -e "${RED}❌ 无效选项${NC}"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}✅ 更新完成！${NC}"
echo ""

# 显示服务状态
echo -e "${BLUE}📊 服务状态:${NC}"
docker-compose ps
echo ""

# 健康检查
echo -e "${BLUE}🏥 健康检查:${NC}"

# 检查后端
if curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 后端健康${NC}"
else
    echo -e "${RED}❌ 后端不健康${NC}"
fi

# 检查前端
if curl -s http://localhost:3000/health > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 前端健康${NC}"
else
    echo -e "${RED}❌ 前端不健康${NC}"
fi

echo ""
echo -e "${BLUE}📝 查看日志:${NC}"
echo "  docker-compose logs -f"
echo ""
echo -e "${BLUE}🌐 访问地址:${NC}"
echo "  前端: http://localhost:3000"
echo "  后端: http://localhost:8080"
echo ""
echo -e "${GREEN}🎉 部署完成！${NC}"



