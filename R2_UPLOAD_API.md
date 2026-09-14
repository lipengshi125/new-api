# Cloudflare R2 文件上传 API 使用文档

## 功能说明

newapi 现已支持 Cloudflare R2 对象存储集成，允许用户通过 API 上传文件到您的 R2 存储桶，并返回自定义域名的公共访问 URL。

## 配置步骤

### 1. 获取 R2 配置信息

登录 Cloudflare Dashboard，进入 R2 页面：

- **Endpoint (端点地址)**: `https://<账号ID>.r2.cloudflarestorage.com`
  - 账号 ID 可在 R2 概览页找到（32位十六进制字符）
  
- **Bucket (存储桶名称)**: 例如 `api`
  - 在 R2 中创建存储桶

- **Access Key ID 和 Secret Access Key**:
  - 进入 R2 → Manage API Tokens
  - 创建新的 API Token
  - **注意**: Secret 只在创建时显示一次，务必保存

- **Public URL (自定义域名)**: 例如 `https://url.yunzao.qzz.io`
  - 在 R2 存储桶设置中绑定自定义域名

### 2. 在 newapi 系统设置中配置 R2

1. 登录 newapi 管理后台
2. 进入「系统设置」页面
3. 找到「Cloudflare R2 存储设置」卡片
4. 填入以下信息：
   - **启用 R2 存储**: 勾选
   - **R2 端点地址**: `https://a407ba32073a1c1037395e891a3958dc.r2.cloudflarestorage.com`
   - **存储桶名称**: `api`
   - **Access Key ID**: `081baaf09f335ebe19fd20da527126d3`
   - **Secret Access Key**: `33924d9f84f1bc7fb2428b313101d1fa289c83588d3af5fa3412b88b807f03ee`
   - **自定义域名**: `https://url.yunzao.qzz.io`
   - **存储路径前缀**: `uploads/` (可选，留空则文件直接保存在桶根目录)
5. 点击「测试连接」验证配置
6. 点击「更新 R2 设置」保存

## API 使用

### 上传文件接口

**请求示例**：

```bash
curl --location --request POST 'https://你的newapi站点/api/media/upload' \
--header 'Authorization: Bearer sk-xxxxxx' \
--form 'file=@"/path/to/your/file.png"'
```

**参数说明**：
- **URL**: `https://你的域名/api/media/upload`
- **Method**: `POST`
- **Headers**: 
  - `Authorization: Bearer <用户的API密钥>`
- **Body**: `multipart/form-data`
  - `file`: 要上传的文件（二进制）

**响应示例**：

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

**响应字段说明**：
- `type`: 文件类型 (`image`, `audio`, `video`, `file`)
- `download_url`: 文件的公共访问 URL（使用您配置的自定义域名）
- `size`: 文件大小（字节）

## 支持的文件类型

系统会自动检测文件的 MIME 类型并分类：
- **image**: `image/*` (图片)
- **audio**: `audio/*` (音频)
- **video**: `video/*` (视频)
- **file**: 其他类型

## 文件命名规则

上传的文件会自动重命名以避免冲突：
- 格式: `原文件名_时间戳.扩展名`
- 示例: `photo.png` → `photo_20260914123045.png`

## 安全说明

1. **认证要求**: 必须使用有效的 API Key 进行身份验证
2. **密钥保护**: R2 的 Access Key ID 和 Secret 存储在数据库中，前端不会显示
3. **权限控制**: 只有通过认证的用户才能上传文件

## 错误处理

常见错误响应：

```json
{
    "code": 1,
    "msg": "R2 storage is not enabled",
    "errorMessages": null,
    "data": null
}
```

可能的错误：
- `unauthorized`: 未提供或无效的 API Key
- `R2 storage is not enabled`: R2 存储未启用
- `file is required`: 未提供文件
- `failed to upload to R2: ...`: 上传到 R2 失败（检查配置和网络）

## 使用场景示例

### 在 Apifox 中测试

1. 创建新请求
2. 设置 URL: `POST https://你的域名/api/media/upload`
3. 添加 Header: `Authorization: Bearer sk-your-api-key`
4. Body 选择 `form-data`
5. 添加字段 `file`，类型选择 `File`，选择要上传的文件
6. 发送请求

### 集成到应用

```javascript
// JavaScript 示例
async function uploadFile(file, apiKey) {
    const formData = new FormData();
    formData.append('file', file);
    
    const response = await fetch('https://your-newapi.com/api/media/upload', {
        method: 'POST',
        headers: {
            'Authorization': `Bearer ${apiKey}`
        },
        body: formData
    });
    
    const result = await response.json();
    if (result.code === 0) {
        console.log('文件URL:', result.data.download_url);
        return result.data.download_url;
    } else {
        throw new Error(result.msg);
    }
}
```

## 注意事项

1. **存储成本**: R2 存储是按用量计费，请注意控制上传量
2. **文件大小限制**: 建议设置合理的文件大小限制
3. **自定义域名**: 必须正确配置 R2 自定义域名并解析，否则返回的 URL 无法访问
4. **CORS 配置**: 如需在浏览器中直接访问，需在 R2 存储桶中配置 CORS 策略

## 技术实现

- **后端**: Go + AWS SDK v2 (S3 兼容 API)
- **存储**: Cloudflare R2 (兼容 S3 协议)
- **认证**: newapi 内置的用户认证系统
