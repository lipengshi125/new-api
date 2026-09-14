@echo off
echo ========================================
echo  New-API Docker 部署脚本
echo ========================================
echo.

echo [1/4] 停止旧容器...
docker-compose -f docker-compose.local.yml down 2>nul

echo [2/4] 拉取镜像...
docker-compose -f docker-compose.local.yml pull

echo [3/4] 构建并启动服务...
docker-compose -f docker-compose.local.yml up -d --build

echo [4/4] 等待服务启动...
timeout /t 10 /nobreak >nul

echo.
echo ========================================
echo  检查服务状态
echo ========================================
docker-compose -f docker-compose.local.yml ps

echo.
echo ========================================
echo  访问地址: http://localhost:3000
echo  默认账号: root
echo  默认密码: 123456
echo ========================================
echo.
echo 查看日志: docker-compose -f docker-compose.local.yml logs -f
echo 停止服务: docker-compose -f docker-compose.local.yml down
echo.
pause
