# 🐳 Docker Desktop 本地部署指南

本指南适用于在本机 Docker Desktop 上部署包含 R2 功能的 newapi。

## 📋 前置要求

- ✅ Docker Desktop 已安装并运行
- ✅ Git 已安装
- ✅ 网络连接正常

## 🚀 快速部署

### 方法一：使用本地构建配置（推荐 - 包含 R2 功能）

```bash
# 1. 进入项目目录
cd c:\Users\adminetetwt\Documents\GitHub\newapi

# 2. 构建并启动所有服务
docker-compose -f docker-compose.local.yml up -d --build

# 3. 查看日志
docker-compose -f docker-compose.local.yml logs -f new-api

# 4. 访问服务
# 浏览器打开: http://localhost:3000
```

### 方法二：使用官方镜像（不包含 R2 功能）

```bash
# 使用原始 docker-compose.yml
docker-compose up -d

# 访问: http://localhost:3000
```

## 📦 服务组成

启动后会运行以下容器：

| 服务 | 容器名 | 端口 | 说明 |
|------|--------|------|------|
| new-api | new-api-local | 3000 | 主应用服务 |
| PostgreSQL | postgres-local | 5432 | 数据库 |
| Redis | redis-local | 6379 | 缓存 |

## 🔧 首次配置

### 1. 访问系统

打开浏览器访问：`http://localhost:3000`

### 2. 默认管理员账号

```
用户名: root
密码: 123456
```

**⚠️ 重要：首次登录后请立即修改密码！**

### 3. 配置 R2 存储

登录后进入「系统设置」：

1. 找到「Cloudflare R2 存储设置」
2. 填入你的 R2 配置：
   - 启用 R2 存储：✅
   - R2 端点地址：`https://<账号ID>.r2.cloudflarestorage.com`
   - 存储桶名称：`api`
   - Access Key ID：`你的 key_id`
   - Secret Access Key：`你的 secret`
   - 自定义域名：`https://url.yunzao.qzz.io`
   - 存储路径前缀：`1day/` (可选)
3. 点击「测试连接」
4. 点击「更新 R2 设置」

### 4. 测试文件上传

```bash
# 创建 API Key（在 Web 界面中：令牌管理 → 创建新令牌）

# 测试上传
curl --location --request POST 'http://localhost:3000/api/media/upload' \
--header 'Authorization: Bearer sk-your-api-key' \
--form 'file=@"C:\path\to\your\file.png"'
```

## 📊 管理命令

### 查看服务状态
```bash
docker-compose -f docker-compose.local.yml ps
```

### 查看日志
```bash
# 查看所有服务日志
docker-compose -f docker-compose.local.yml logs -f

# 只看 new-api 日志
docker-compose -f docker-compose.local.yml logs -f new-api

# 查看最近 100 行
docker-compose -f docker-compose.local.yml logs --tail=100 new-api
```

### 重启服务
```bash
# 重启所有服务
docker-compose -f docker-compose.local.yml restart

# 只重启 new-api
docker-compose -f docker-compose.local.yml restart new-api
```

### 停止服务
```bash
# 停止但保留数据
docker-compose -f docker-compose.local.yml stop

# 停止并删除容器（数据卷保留）
docker-compose -f docker-compose.local.yml down

# 停止并删除所有内容（包括数据卷）⚠️
docker-compose -f docker-compose.local.yml down -v
```

### 重新构建
```bash
# 代码更新后重新构建
docker-compose -f docker-compose.local.yml up -d --build --force-recreate
```

## 🔍 故障排查

### 问题 1：端口被占用

**错误信息**：`Bind for 0.0.0.0:3000 failed: port is already allocated`

**解决方法**：
```bash
# 查看占用端口的进程
netstat -ano | findstr :3000

# 修改 docker-compose.local.yml 中的端口映射
# 将 "3000:3000" 改为 "3001:3000"
# 然后访问 http://localhost:3001
```

### 问题 2：构建失败

**可能原因**：网络问题或 Docker 资源不足

**解决方法**：
```bash
# 1. 清理 Docker 缓存
docker system prune -a

# 2. 增加 Docker Desktop 资源
# Docker Desktop → Settings → Resources
# 增加 CPU 和内存配额

# 3. 重试构建
docker-compose -f docker-compose.local.yml build --no-cache
```

### 问题 3：数据库连接失败

**检查方法**：
```bash
# 查看 postgres 容器日志
docker-compose -f docker-compose.local.yml logs postgres

# 进入 postgres 容器测试
docker exec -it postgres-local psql -U root -d new-api
```

### 问题 4：R2 上传失败

**检查清单**：
- ✅ R2 配置是否正确（endpoint、bucket、key、secret）
- ✅ R2 存储桶是否已创建
- ✅ API Token 权限是否包含 R2 写入
- ✅ 自定义域名是否已绑定并解析
- ✅ 网络是否能访问 Cloudflare R2

**测试连接**：
```bash
# 在系统设置中点击「测试连接」按钮
# 或查看 new-api 容器日志
docker-compose -f docker-compose.local.yml logs -f new-api | grep -i r2
```

## 📁 数据持久化

数据存储位置：

```
项目目录/
├── data/                # SQLite 数据文件（如果使用 SQLite）
├── logs/                # 应用日志
└── Docker Volume:
    └── pg_data_local    # PostgreSQL 数据（Docker 卷）
```

### 备份数据库

```bash
# PostgreSQL 备份
docker exec postgres-local pg_dump -U root new-api > backup_$(date +%Y%m%d_%H%M%S).sql

# 恢复
docker exec -i postgres-local psql -U root new-api < backup.sql
```

## 🔐 安全建议

在生产环境部署前：

1. ✅ 修改所有默认密码
   - 数据库密码（`docker-compose.local.yml` 中的 `POSTGRES_PASSWORD` 和 `SQL_DSN`）
   - Redis 密码（`redis-server --requirepass` 和 `REDIS_CONN_STRING`）
   - 管理员账号密码
   
2. ✅ 配置 HTTPS
   - 使用 Nginx 或 Caddy 作为反向代理
   - 配置 SSL 证书
   
3. ✅ 限制外部访问
   - 不要暴露 PostgreSQL 和 Redis 端口
   - 只暴露必要的端口（3000）

4. ✅ 定期备份数据

5. ✅ 监控日志和资源使用

## 🌐 外网访问

如果需要外网访问（例如 API 调用）：

### 方法 1：使用 ngrok（临时测试）

```bash
# 安装 ngrok
# 下载: https://ngrok.com/download

# 启动隧道
ngrok http 3000

# 会得到一个公网地址如: https://xxxx.ngrok.io
```

### 方法 2：配置路由器端口转发

1. 路由器管理界面 → 端口转发
2. 添加规则：外部端口 3000 → 内网 IP:3000
3. 配置动态域名（如 DDNS）

### 方法 3：部署到云服务器

- 阿里云、腾讯云、AWS 等
- 配置安全组规则开放 3000 端口

## 📚 更多信息

- 完整 API 文档：查看项目中的 `R2_UPLOAD_API.md`
- GitHub 仓库：https://github.com/QuantumNous/new-api
- 问题反馈：提交 GitHub Issue

## ✨ 快速命令参考

```bash
# 启动服务
docker-compose -f docker-compose.local.yml up -d --build

# 查看日志
docker-compose -f docker-compose.local.yml logs -f new-api

# 重启服务
docker-compose -f docker-compose.local.yml restart new-api

# 停止服务
docker-compose -f docker-compose.local.yml down

# 查看服务状态
docker-compose -f docker-compose.local.yml ps

# 进入容器
docker exec -it new-api-local sh

# 备份数据库
docker exec postgres-local pg_dump -U root new-api > backup.sql
```
