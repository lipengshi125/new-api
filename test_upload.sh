#!/bin/bash

# 测试文件上传到 R2
API_URL="https://api.cdmai.top"
API_KEY="sk-P7Hop9QbRFHrarQq5PX08pydHI6N99SDOAAFY6UtAZnbIxgf"
TEST_FILE="E:\\xiazai\\vosr2_upscale_00001_txfgb_1789219888.jfif"

echo "=== 步骤 1: 验证 token 是否有效 ==="
echo "请求: GET ${API_URL}/api/user/self"
USER_RESPONSE=$(curl -s -X GET "${API_URL}/api/user/self" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Accept: application/json")

echo "响应:"
echo "$USER_RESPONSE" | jq . 2>/dev/null || echo "$USER_RESPONSE"

echo ""
echo "=== 步骤 2: 检查 R2 配置状态 ==="
echo "请求: POST ${API_URL}/api/r2/test"
R2_TEST=$(curl -s -X POST "${API_URL}/api/r2/test" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Accept: application/json")

echo "响应:"
echo "$R2_TEST" | jq . 2>/dev/null || echo "$R2_TEST"

echo ""
echo "=== 步骤 3: 上传文件到 R2 ==="
echo "请求: POST ${API_URL}/api/media/upload"
echo "文件: ${TEST_FILE}"

UPLOAD_RESPONSE=$(curl -s -X POST "${API_URL}/api/media/upload" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Accept: application/json" \
  -F "file=@${TEST_FILE}")

echo "响应:"
echo "$UPLOAD_RESPONSE" | jq . 2>/dev/null || echo "$UPLOAD_RESPONSE"

echo ""
echo "=== 完成 ==="
