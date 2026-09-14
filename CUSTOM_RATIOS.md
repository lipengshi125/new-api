# 自定义倍率功能 (Custom Ratios)

## 功能概述

自定义倍率功能允许管理员为模型定义自定义的计费参数，并根据请求中的参数值动态应用不同的倍率。

## 使用场景

- **分辨率计费**: 根据图片/视频分辨率（1k/2k/4k）应用不同倍率
- **质量等级**: 根据输出质量（low/medium/high）调整价格
- **任意自定义参数**: 支持从请求的 JSON body、form-data 或 query 参数中提取任意字段

## 配置方法

### 1. 通过配置文件

编辑 `config/model_ratio.json`，在模型配置中添加 `custom_ratios` 字段：

```json
{
  "sora-1.0": 1.5,
  "custom_ratios": {
    "sora-1.0": {
      "resolution": {
        "1080p": 1.0,
        "2k": 1.2,
        "4k": 1.5,
        "8k": 2.0
      },
      "quality": {
        "draft": 0.8,
        "standard": 1.0,
        "high": 1.3
      }
    }
  }
}
```

### 2. 通过 API

**获取模型定价配置**:
```bash
GET /api/pricing?model=sora-1.0
```

响应示例：
```json
{
  "model_name": "sora-1.0",
  "model_ratio": 1.5,
  "custom_ratios": {
    "resolution": {
      "1080p": 1.0,
      "2k": 1.2,
      "4k": 1.5
    }
  }
}
```

**更新自定义倍率**:
```bash
POST /api/pricing
Content-Type: application/json

{
  "models": ["sora-1.0"],
  "custom_ratios": {
    "resolution": {
      "1080p": 1.0,
      "2k": 1.2,
      "4k": 1.5
    }
  }
}
```

## 工作原理

### 1. 参数提取

系统会自动从以下位置提取参数（按优先级）：

1. **JSON body** 的一级字段
2. **Multipart form-data** 的字段
3. **URL query** 参数

示例请求：
```json
POST /v1/video/generations
{
  "model": "sora-1.0",
  "prompt": "A beautiful sunset",
  "resolution": "4k",
  "quality": "high"
}
```

### 2. 倍率应用

系统会匹配 `custom_ratios` 配置：

- `resolution = "4k"` → 应用倍率 `1.5`
- `quality = "high"` → 应用倍率 `1.3`
- 最终倍率 = 基础倍率 × 1.5 × 1.3 = `1.5 × 1.5 × 1.3 = 2.925`

### 3. 计费流程

```
基础价格 (model_ratio) 
  → 应用自定义倍率 (custom_ratios)
  → 应用时长倍率 (seconds/duration, 仅按秒计费模型)
  → 应用分组倍率 (group_ratio)
  → 最终扣费
```

## 日志示例

启用后，日志会显示应用的自定义倍率：

```
[DEBUG] Applied custom ratio: resolution=4k -> x1.50
[DEBUG] Applied custom ratio: quality=high -> x1.30
[INFO] 计算参数: resolution: 1.50, quality: 1.30
```

## 注意事项

### 1. 参数提取限制

- 仅支持**一级字段**（不支持嵌套对象）
- 参数值会转换为字符串进行匹配
- 匹配区分大小写

### 2. 倍率计算规则

- 所有匹配的倍率会**累乘**
- 未匹配的参数不影响计费
- 倍率必须 > 0，NaN 和 Inf 会被拒绝

### 3. 与其他计费模式的关系

- **按次计费**: 自定义倍率生效，时长倍率不生效
- **按秒计费**: 自定义倍率 + 时长倍率都生效
- **按 token 计费**: 自定义倍率 + token 重算都生效

### 4. 更新生效时间

- 配置文件修改后需重启服务
- API 更新后立即生效（内存缓存自动刷新）

## 最佳实践

### 1. 命名规范

建议使用清晰的参数名：
- `resolution` 而非 `res` 或 `r`
- `quality` 而非 `q`
- `output_format` 而非 `fmt`

### 2. 倍率设置

建议设置合理的倍率范围：
- 基准倍率: `1.0`
- 常用范围: `0.5 ~ 3.0`
- 避免过大倍率（如 > 10.0）

### 3. 测试验证

配置后建议测试：
```bash
# 测试请求
curl -X POST http://localhost:3000/v1/video/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "sora-1.0",
    "prompt": "test",
    "resolution": "4k"
  }'

# 检查使用日志
# 确认 "计算参数" 显示了 resolution 倍率
```

## 故障排查

### 倍率未生效

1. 检查参数名是否匹配（区分大小写）
2. 检查参数是否在请求的一级字段
3. 检查日志是否有 "Applied custom ratio" 消息

### 倍率不符合预期

1. 检查是否有多个倍率被累乘
2. 检查是否与时长倍率冲突（按次计费不应有时长倍率）
3. 查看使用日志中的 "计算参数" 部分

### API 更新失败

1. 检查 JSON 格式是否正确
2. 检查倍率值是否 > 0
3. 检查模型名称是否存在
