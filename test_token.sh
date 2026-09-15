#!/bin/bash

# 测试脚本：验证 API Key 是否有效

API_URL="https://api.cdmai.top"
API_KEY="sk-X3S3Wklk048elIMBk78JFNORrvz757s0p1ZYQjpOFICAX1US"

echo "=== 测试 1: 检查 API Key 是否有效 ==="
curl -s -X GET "${API_URL}/api/user/self" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Accept: application/json" | jq .

echo ""
echo "=== 测试 2: 尝试上传文件 ==="
curl -s -X POST "${API_URL}/api/media/upload" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Accept: application/json" \
  -F "file=@E:\\xiazai\\vosr2_upscale_00001_txfgb_1789219888.jfif" | jq .

echo ""
echo "=== 如果上面都返回 'Unauthorized'，说明 token 无效，需要重新生成 ==="
