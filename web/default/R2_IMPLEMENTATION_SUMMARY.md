# Cloudflare R2 文件上传功能实现总结

## 📦 完成的工作

已成功为 newapi 项目实现完整的 Cloudflare R2 对象存储集成功能，用户可以通过 API 上传文件到 R2 存储桶，并获得自定义域名的公共访问 URL。

---

## 🎯 实现的功能

### 1. **后端实现**

#### 新增文件

**`service/r2_storage.go`** - R2 存储服务
- 使用 AWS SDK v2 (S3 兼容 API) 连接 Cloudflare R2
- 实现文件上传功能 (`UploadFile`)
- 支持自动文件类型检测 (image/audio/video/file)
- 自动生成唯一文件名（原文件名_时间戳）
- 支持自定义存储路径前缀
- 返回自定义域名的公共 URL

**`controller/media.go`** - 文件上传控制器
- `/api/media/upload` - 用户文件上传接口（需要 API Key 认证）
- `/api/r2/test` - R2 连接测试接口（管理员）
- `/api/r2/settings` - R2 配置更新接口（管理员）

#### 修改文件

**`model/option.go`**
- 添加 R2 配置字段到系统设置：
  - `R2StorageEnabled` - 启用开关
  - `R2Endpoint` - R2 端点地址
  - `R2Bucket` - 存储桶名称
  - `R2KeyID` - Access Key ID
  - `R2Secret` - Secret Access Key
  - `R2PublicURL` - 自定义域名
  - `R2StoragePath` - 存储路径前缀

**`router/api-router.go`**
- 注册媒体上传路由组：
  - `POST /api/media/upload` - 文件上传（需要认证）
- 注册 R2 管理路由组（管理员权限）：
  - `POST /api/r2/test` - 测试连接
  - `PUT /api/r2/settings` - 更新配置

**`go.mod` & `go.sum`**
- 添加 AWS SDK v2 依赖：
  - `github.com/aws/aws-sdk-go-v2`
  - `github.com/aws/aws-sdk-go-v2/config`
  - `github.com/aws/aws-sdk-go-v2/credentials`
  - `github.com/aws/aws-sdk-go-v2/service/s3`

---

### 2. **前端实现**

#### Classic 前端 (`web/classic`)

**`web/classic/src/components/settings/SystemSetting.jsx`**
- 添加「Cloudflare R2 存储设置」卡片
- 表单字段：
  - 启用 R2 存储（开关）
  - R2 端点地址（URL 输入）
  - 存储桶名称（文本输入）
  - Access Key ID（密码输入）
  - Secret Access Key（密码输入）
  - 自定义域名（URL 输入）
  - 存储路径前缀（文本输入）
- 操作按钮：
  - 「测试连接」- 验证 R2 配置
  - 「更新 R2 设置」- 保存配置

#### Default 前端 (`web/default`)

**新增文件：**
- `web/default/src/features/system-settings/integrations/r2-storage-settings-section.tsx`
  - React 组件，使用 React Hook Form + Zod 验证
  - 完整的表单验证和错误提示
  - 集成 toast 通知
  - 支持测试连接和保存配置

**修改文件：**
- `web/default/src/features/system-settings/operations/section-registry.tsx`
  - 导入 R2StorageSettingsSection
  - 注册 'r2-storage' 配置节
  - 在 Worker 配置之后显示

- `web/default/src/features/system-settings/types.ts`
  - 添加 R2 配置字段到 `OperationsSettings` 类型定义

---

## 📝 API 使用说明

### 上传文件接口

#### 请求示例

```bash
curl --location --request POST 'https://你的newapi站点/api/media/upload' \
--header 'Authorization: Bearer sk-xxxxxx' \
--form 'file=@"/path/to/file.png"'
```

#### 响应示例

```json
{
    "code": 0,
    "msg": "success",
    "errorMessages": null,
    "data": {
        "type": "image",
        "download_url": "https://url.yunzao.qzz.io/uploads/filename_20260914123045.png",
        "size": "7703949"
    }
}
```

#### 响应字段说明

- `type`: 文件类型（`image` / `audio` / `video` / `file`）
- `download_url`: 文件的公共访问 URL（使用您的自定义域名）
- `size`: 文件大小（字节）

---

## 🔧 配置步骤

### 1. 获取 Cloudflare R2 配置信息

登录 Cloudflare Dashboard → R2：

1. **账号 ID**：在 R2 概览页找到（32位十六进制）
2. **创建存储桶**：例如命名为 `api`
3. **创建 API Token**：
   - 进入 R2 → Manage API Tokens
   - 创建新 token
   - **保存 Access Key ID 和 Secret**（Secret 只显示一次）
4. **绑定自定义域名**：
   - 在存储桶设置中绑定域名（如 `https://url.yunzao.qzz.io`）
   - 配置 DNS 解析

### 2. 在 newapi 系统设置中配置

#### 方式一：Classic 前端
1. 登录管理后台 → 系统设置
2. 找到「Cloudflare R2 存储设置」卡片
3. 填写配置信息：
   - **启用 R2 存储**：勾选
   - **R2 端点地址**：`https://a407ba32073a1c1037395e891a3958dc.r2.cloudflarestorage.com`
   - **存储桶名称**：`api`
   - **Access Key ID**：你的 Key ID
   - **Secret Access Key**：你的 Secret
   - **自定义域名**：`https://url.yunzao.qzz.io`
   - **存储路径前缀**：`1day/`（可选，留空则保存在根目录）
4. 点击「测试连接」验证
5. 点击「更新 R2 设置」保存

#### 方式二：Default 前端
1. 登录管理后台 → 系统设置 → Operations
2. 找到「Cloudflare R2 Storage」节
3. 填写配置信息（字段同上）
4. 点击「Test Connection」验证
5. 点击保存按钮

### 3. 更换 Cloudflare 账号

如需更换账号，只需在系统设置中修改以下字段：

- **R2 端点地址**：改为新账号的 `https://<新账号ID>.r2.cloudflarestorage.com`
- **存储桶名称**：新账号的存储桶名
- **Access Key ID**：新账号的 Key ID
- **Secret Access Key**：新账号的 Secret
- **自定义域名**：新账号绑定的域名
- **存储路径前缀**：按需修改（如 `1day/`）

**提示**：
- ✅ 配置修改后立即生效，无需重启服务
- ✅ 旧文件继续保留在旧账号中
- ✅ 新上传的文件自动使用新配置

---

## ✨ 核心特性

### 安全性
- ✅ **用户认证**：必须使用有效的 API Key 才能上传
- ✅ **密钥保护**：R2 密钥存储在数据库，前端不显示明文
- ✅ **权限控制**：只有管理员能配置 R2 设置

### 文件处理
- ✅ **自动类型检测**：根据 MIME 类型识别文件类型
- ✅ **唯一命名**：时间戳命名避免冲突 `filename_20260914123045.ext`
- ✅ **路径组织**：支持可选的存储路径前缀
- ✅ **自定义域名**：返回您自己的域名 URL

### 用户体验
- ✅ **连接测试**：保存前验证配置是否正确
- ✅ **实时反馈**：成功/失败即时通知
- ✅ **热更新**：配置修改立即生效
- ✅ **双前端支持**：Classic 和 Default 前端都已适配

---

## 📂 文件清单

### 后端文件
```
新增:
  controller/media.go              # 文件上传控制器
  service/r2_storage.go            # R2 存储服务

修改:
  model/option.go                  # 添加 R2 配置选项
  router/api-router.go             # 注册 API 路由
  go.mod                           # 添加 AWS SDK 依赖
  go.sum                           # 依赖锁文件
```

### 前端文件 (Classic)
```
修改:
  web/classic/src/components/settings/SystemSetting.jsx
```

### 前端文件 (Default)
```
新增:
  web/default/src/features/system-settings/integrations/r2-storage-settings-section.tsx

修改:
  web/default/src/features/system-settings/operations/section-registry.tsx
  web/default/src/features/system-settings/types.ts
```

### 文档
```
新增:
  R2_UPLOAD_API.md                 # API 使用文档
  R2_IMPLEMENTATION_SUMMARY.md     # 实现总结（本文档）
```

---

## 🚀 部署和测试

### 1. 编译项目

```bash
# 安装 Go 依赖
go mod tidy

# 构建后端
go build

# 构建前端 (Default)
cd web/default
bun install
bun run build
cd ../..

# 构建前端 (Classic) - 如果需要
cd web/classic
npm install
npm run build
cd ../..
```

### 2. 启动服务

```bash
./newapi
```

### 3. 配置 R2

访问管理后台 → 系统设置，配置 R2 信息。

### 4. 测试上传

```bash
curl --location --request POST 'http://localhost:3000/api/media/upload' \
--header 'Authorization: Bearer sk-your-api-key' \
--form 'file=@"test.png"'
```

---

## 📊 技术栈

- **后端**：Go 1.22+, Gin, GORM
- **存储**：Cloudflare R2 (S3 兼容)
- **SDK**：AWS SDK for Go v2
- **前端 (Classic)**：React 18, Semi Design
- **前端 (Default)**：React 19, Base UI, Tailwind CSS, React Hook Form, Zod

---

## 🔍 注意事项

1. **存储成本**：R2 按用量计费，请注意控制上传量
2. **文件大小**：建议在代码中添加文件大小限制
3. **CORS 配置**：如需浏览器直接访问，需在 R2 配置 CORS
4. **自定义域名**：必须正确配置并解析，否则 URL 无法访问
5. **密钥安全**：Secret Access Key 只在创建时显示一次，务必保存

---

## ✅ 验证清单

- [x] 后端 R2 存储服务实现
- [x] 后端文件上传 API
- [x] 后端 R2 管理 API
- [x] 系统设置数据模型更新
- [x] API 路由注册
- [x] Classic 前端配置界面
- [x] Default 前端配置界面
- [x] 前端类型定义
- [x] Go 依赖更新
- [x] 前端编译通过
- [x] API 使用文档
- [x] 实现总结文档

---

## 📞 使用场景

### 1. 图片托管
用户上传图片，获得 CDN 加速的公共 URL，用于网站、博客、应用等。

### 2. 文件分享
上传文件到 R2，生成链接分享给他人。

### 3. API 集成
第三方应用通过 API Key 上传文件，无需直接管理存储。

### 4. 临时存储
配合路径前缀（如 `1day/`）实现分类管理，方便后续清理。

---

## 🎉 总结

本次实现完整地为 newapi 项目集成了 Cloudflare R2 对象存储功能：

1. ✅ **完整的后端服务**：文件上传、配置管理、连接测试
2. ✅ **双前端支持**：Classic 和 Default 前端都有配置界面
3. ✅ **安全可靠**：认证、密钥保护、权限控制
4. ✅ **易于使用**：简单的 API，直观的配置界面
5. ✅ **灵活配置**：支持更换账号、自定义路径、自定义域名

用户现在可以通过 API 上传文件到自己的 R2 存储桶，并使用自己的域名返回 URL，完美解决了文件存储和分发的需求！🚀
