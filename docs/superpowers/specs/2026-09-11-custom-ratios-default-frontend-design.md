# 新版前端自定义倍率参数配置界面设计

**日期：** 2026-09-11  
**状态：** 已批准  
**实现范围：** web/default 前端管理界面

## 概述

为新版前端（web/default）添加自定义倍率参数的配置界面，使管理员能够通过现代化的表格界面配置请求参数对应的价格倍率。当前经典版前端已实现此功能，后端已完全支持，本设计专注于新版前端的管理界面实现。

## 背景

### 已完成部分
- ✅ 后端自定义倍率系统完整实现
- ✅ 后端 API 支持保存和加载 CustomRatios 配置
- ✅ 经典版前端管理界面已实现
- ✅ 新版前端用户查看价格时可以看到自定义倍率参数

### 待实现部分
- ❌ 新版前端管理员配置自定义倍率参数的界面

## 设计目标

1. 提供直观的表格式界面用于配置自定义倍率参数
2. 支持添加、编辑、删除参数-值-倍率映射
3. 实时验证输入，防止无效配置
4. 与新版前端的现代化 UI 风格保持一致
5. 数据格式与后端 API 完全兼容

## 架构设计

### 组件结构

```
ModelPricingEditorPanel (修改)
├── 模型名称输入
├── 计费模式标签页 (Tabs)
│   ├── 按 Token
│   ├── 按次
│   └── 动态计费
├── CustomRatiosTable (新增)
│   ├── 标题与说明
│   ├── 添加按钮
│   └── 数据表格
│       ├── 参数名列
│       ├── 参数值列
│       ├── 倍率列
│       └── 操作列
└── 保存按钮
```

### 文件变更

**新增文件：**
- `web/default/src/features/system-settings/models/custom-ratios-table.tsx`

**修改文件：**
- `web/default/src/features/system-settings/models/model-pricing-sheet.tsx`
- `web/default/src/features/system-settings/models/model-pricing-core.ts` (已完成 - 添加类型定义)

## 功能设计

### 1. 集成位置

自定义倍率参数编辑区域放置在**计费模式标签页之后，保存按钮之前**，作为通用配置区域。

**条件显示：**
- 在"按 Token"和"按次"计费模式下显示
- 在"动态计费"模式下隐藏（动态计费使用表达式，自定义倍率可能冲突）

### 2. 数据结构

```typescript
// TypeScript 类型定义
type CustomRatios = Record<string, Record<string, number>>

// 示例数据
{
  "resolution": {
    "1k": 1.0,
    "2k": 1.2,
    "4k": 1.5
  },
  "quality": {
    "low": 0.8,
    "medium": 1.0,
    "high": 1.3
  }
}
```

### 3. CustomRatiosTable 组件设计

#### 3.1 UI 布局

**标题区：**
- 标题："自定义倍率参数"
- 说明文字："配置请求参数对应的价格倍率，多个参数倍率会相乘叠加"

**工具栏：**
- 主按钮："+ 添加参数"
- 位置：表格上方右侧

**表格结构：**
| 参数名 | 参数值 | 倍率 | 操作 |
|--------|--------|------|------|
| resolution | 1k | 1.0 | 🗑️ |
| ↳ | 2k | 1.2 | 🗑️ |
| ↳ | 4k | 1.5 | 🗑️ |
| quality | low | 0.8 | 🗑️ |
| ↳ | medium | 1.0 | 🗑️ |
| ↳ | high | 1.3 | 🗑️ |

**行分组显示：**
- 相同参数名的行使用浅色背景（`bg-muted/30`）
- 第一行显示完整参数名
- 后续同参数的行显示"↳"符号表示从属关系

#### 3.2 交互流程

**添加新参数：**
1. 点击"添加参数"按钮
2. 表格新增一行，三个输入框为空且可编辑
3. 参数名输入框自动聚焦
4. 用户依次输入参数名、参数值、倍率
5. 实时验证输入

**添加同参数的新值：**
1. 在已有参数名的输入框中输入相同的参数名
2. 系统自动将其分组到已有参数下
3. 视觉上显示为从属行

**删除参数值：**
1. 点击行末的删除按钮
2. 该行立即移除
3. 如果删除后参数名下没有任何值，整个参数被移除

**编辑现有值：**
1. 直接点击输入框编辑
2. 实时验证
3. 失去焦点后保存更改

### 4. 输入验证

#### 4.1 参数名验证
- **必填**
- **格式：** 正则表达式 `/^[a-zA-Z0-9_-]+$/`
- **允许：** 英文字母、数字、下划线、连字符
- **错误提示：** "参数名只能包含字母、数字、下划线和连字符"

#### 4.2 参数值验证
- **必填**
- **允许：** 任意字符（包括中文、空格、特殊符号）
- **最大长度：** 100 字符
- **错误提示：** "参数值不能为空"

#### 4.3 倍率验证
- **必填**
- **类型：** 正数（number > 0）
- **格式：** 支持小数，最多 4 位小数
- **错误提示：** "倍率必须大于 0"

#### 4.4 唯一性检查
- **检查：** 相同"参数名 + 参数值"组合
- **行为：** 显示警告图标和提示文字
- **提示：** "此参数值已存在，会覆盖之前的配置"
- **允许保存：** 是（警告不阻止保存）

#### 4.5 保存前验证
- 所有行必须通过基本验证（必填、格式）
- 如有验证错误，焦点定位到第一个错误输入框
- 显示总体错误提示："请修正所有验证错误后再保存"

### 5. 状态管理

#### 5.1 组件状态

在 `ModelPricingEditorPanel` 中添加：

```typescript
const [customRatios, setCustomRatios] = useState<Record<string, Record<string, number>>>({})
```

#### 5.2 数据加载

在 `useEffect` 中，从 `editData` 加载现有配置：

```typescript
useEffect(() => {
  if (editData?.customRatios) {
    setCustomRatios(editData.customRatios)
  } else {
    setCustomRatios({})
  }
}, [editData])
```

#### 5.3 数据提交

在 `commitDraft` 函数中，将 `customRatios` 包含到返回数据：

```typescript
const commitDraft = async () => {
  // ...现有验证逻辑...
  
  return {
    name: form.getValues('name'),
    // ...其他字段...
    customRatios: Object.keys(customRatios).length > 0 ? customRatios : undefined,
  }
}
```

### 6. 数据转换

#### 6.1 内部格式到扁平列表

CustomRatiosTable 内部使用扁平列表结构便于渲染：

```typescript
type FlatRatioRow = {
  id: string // 唯一标识
  paramName: string
  paramValue: string
  ratio: number
}
```

转换函数：
```typescript
function flattenCustomRatios(ratios: CustomRatios): FlatRatioRow[] {
  const rows: FlatRatioRow[] = []
  Object.entries(ratios).forEach(([paramName, valueMap]) => {
    Object.entries(valueMap).forEach(([paramValue, ratio]) => {
      rows.push({
        id: `${paramName}::${paramValue}`,
        paramName,
        paramValue,
        ratio,
      })
    })
  })
  return rows
}
```

#### 6.2 扁平列表到嵌套格式

保存时转换回后端格式：

```typescript
function unflattenCustomRatios(rows: FlatRatioRow[]): CustomRatios {
  const ratios: CustomRatios = {}
  rows.forEach((row) => {
    if (!ratios[row.paramName]) {
      ratios[row.paramName] = {}
    }
    ratios[row.paramName][row.paramValue] = row.ratio
  })
  return ratios
}
```

## 技术实现细节

### 组件 Props

```typescript
type CustomRatiosTableProps = {
  value: Record<string, Record<string, number>>
  onChange: (value: Record<string, Record<string, number>>) => void
}
```

### UI 组件库

使用新版前端现有的组件：
- `Button` - 添加按钮和删除按钮
- `Input` - 文本输入框
- `Alert` - 说明文字和警告提示
- `Table` - 数据表格（或使用 div 布局模拟表格）
- Lucide 图标库：`Plus`（添加）、`Trash2`（删除）、`AlertTriangle`（警告）

### 样式约定

- 分组行背景：`bg-muted/30`
- 错误输入框边框：`border-red-500`
- 警告图标颜色：`text-amber-500`
- 从属行符号：`text-muted-foreground`

## 国际化

需要添加的翻译键（`web/default/src/i18n/locales/*.json`）：

```json
{
  "Custom Parameter Multipliers": "自定义倍率参数",
  "Configure request parameter multipliers. Multiple multipliers will be multiplied together.": "配置请求参数对应的价格倍率，多个参数倍率会相乘叠加。",
  "Add Parameter": "添加参数",
  "Parameter Name": "参数名",
  "Parameter Value": "参数值",
  "Multiplier": "倍率",
  "Actions": "操作",
  "Parameter name can only contain letters, numbers, underscores and hyphens": "参数名只能包含字母、数字、下划线和连字符",
  "Parameter value is required": "参数值不能为空",
  "Multiplier must be greater than 0": "倍率必须大于 0",
  "This parameter value already exists and will override the previous configuration": "此参数值已存在，会覆盖之前的配置",
  "Please fix all validation errors before saving": "请修正所有验证错误后再保存"
}
```

## 错误处理

### 验证错误
- 输入框显示红色边框
- 错误提示文字显示在输入框下方
- 保存时焦点定位到第一个错误

### 警告
- 重复"参数名+参数值"显示警告图标
- 不阻止保存，仅提示用户

### 边界情况
- 空表格时显示"暂无自定义倍率参数"占位文字
- 删除最后一个参数值后，表格恢复空状态
- 切换到动态计费模式时，自定义倍率数据保留（仅隐藏 UI）

## 测试考虑

### 功能测试
1. 添加新参数 - 验证数据正确保存
2. 添加同参数的多个值 - 验证分组显示
3. 编辑现有值 - 验证更新生效
4. 删除参数值 - 验证数据正确移除
5. 输入验证 - 验证所有验证规则生效
6. 重复检查 - 验证警告正确显示
7. 数据加载 - 验证编辑模式正确加载现有配置
8. 数据提交 - 验证保存到后端的格式正确

### UI 测试
1. 行分组视觉效果
2. 响应式布局
3. 输入框焦点管理
4. 错误提示样式
5. 国际化文字显示

## 实现顺序

1. 修改 `model-pricing-core.ts` 添加辅助函数（flatten/unflatten）
2. 创建 `custom-ratios-table.tsx` 基础组件结构
3. 实现表格渲染和行分组逻辑
4. 实现添加、删除操作
5. 实现输入验证逻辑
6. 集成到 `model-pricing-sheet.tsx`
7. 添加国际化翻译
8. 测试与调优

## 成功标准

- ✅ 管理员可以在新版前端添加、编辑、删除自定义倍率参数
- ✅ 相同参数名的值正确分组显示
- ✅ 输入验证实时生效并显示清晰的错误提示
- ✅ 重复的"参数名+参数值"组合显示警告
- ✅ 保存的数据格式与后端 API 兼容
- ✅ 切换计费模式时 UI 正确显示/隐藏
- ✅ 编辑模式正确加载现有配置
- ✅ UI 风格与新版前端整体设计一致

## 未来扩展

- 批量导入/导出功能（JSON 格式）
- 参数名和参数值的自动补全建议
- 预设模板（常见参数配置）
- 参数值的正则表达式匹配支持
