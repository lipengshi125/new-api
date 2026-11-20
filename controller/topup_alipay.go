package controller

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	alipayGatewayProd    = "https://openapi.alipay.com/gateway.do"
	alipayGatewaySandbox = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
	alipayProductCode    = "FAST_INSTANT_TRADE_PAY"
	alipayPayMethod      = "alipay.trade.page.pay"
)

// 支付宝报文时间使用北京时间
var alipayLocation = time.FixedZone("CST", 8*3600)

type AlipayPayRequest struct {
	Amount int64 `json:"amount"`
}

func isAlipayTopUpEnabled() bool {
	return isPaymentComplianceConfirmed() &&
		setting.AlipayEnabled &&
		strings.TrimSpace(setting.AlipayAppId) != "" &&
		strings.TrimSpace(setting.AlipayPrivateKey) != "" &&
		strings.TrimSpace(setting.AlipayPublicKey) != ""
}

// 支持 PEM 格式，也支持支付宝控制台复制出的无头尾 base64 正文
func alipayKeyDER(raw string) ([]byte, error) {
	s := strings.TrimSpace(raw)
	if block, _ := pem.Decode([]byte(s)); block != nil {
		return block.Bytes, nil
	}
	return base64.StdEncoding.DecodeString(strings.Join(strings.Fields(s), ""))
}

func parseAlipayPrivateKey(raw string) (*rsa.PrivateKey, error) {
	der, err := alipayKeyDER(raw)
	if err != nil {
		return nil, err
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("alipay private key is not RSA")
		}
		return rsaKey, nil
	}
	return x509.ParsePKCS1PrivateKey(der)
}

func parseAlipayPublicKey(raw string) (*rsa.PublicKey, error) {
	der, err := alipayKeyDER(raw)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("alipay public key is not RSA")
	}
	return rsaKey, nil
}

// 按支付宝规则拼接待签名串：去掉 sign 等排除项及空值，按参数名升序排列
func alipaySignContent(params map[string]string, exclude ...string) string {
	skip := make(map[string]bool, len(exclude))
	for _, k := range exclude {
		skip[k] = true
	}
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if skip[k] || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

func signAlipayParams(params map[string]string, key *rsa.PrivateKey) (string, error) {
	digest := sha256.Sum256([]byte(alipaySignContent(params, "sign")))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func verifyAlipayNotify(params map[string]string, pub *rsa.PublicKey) error {
	if params["sign_type"] != "RSA2" {
		return errors.New("unsupported sign_type")
	}
	sig, err := base64.StdEncoding.DecodeString(params["sign"])
	if err != nil {
		return err
	}
	// 支付宝异步通知验签：排除 sign 和 sign_type
	digest := sha256.Sum256([]byte(alipaySignContent(params, "sign", "sign_type")))
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig)
}

func buildAlipayPayURL(tradeNo string, totalAmount string, notifyURL string, returnURL string) (string, error) {
	priv, err := parseAlipayPrivateKey(setting.AlipayPrivateKey)
	if err != nil {
		return "", err
	}
	bizContent, err := common.Marshal(map[string]string{
		"out_trade_no": tradeNo,
		"total_amount": totalAmount,
		"subject":      "TUC" + totalAmount,
		"product_code": alipayProductCode,
	})
	if err != nil {
		return "", err
	}
	params := map[string]string{
		"app_id":      setting.AlipayAppId,
		"method":      alipayPayMethod,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().In(alipayLocation).Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  notifyURL,
		"return_url":  returnURL,
		"biz_content": string(bizContent),
	}
	sign, err := signAlipayParams(params, priv)
	if err != nil {
		return "", err
	}
	params["sign"] = sign

	gateway := alipayGatewayProd
	if setting.AlipaySandbox {
		gateway = alipayGatewaySandbox
	}
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return gateway + "?" + q.Encode(), nil
}

func RequestAlipayPay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	if !isAlipayTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付宝支付未启用"})
		return
	}

	var req AlipayPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if req.Amount < getMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getMinTopup())})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	callBackAddress := service.GetCallbackAddress()
	notifyURL := callBackAddress + "/api/user/payment/alipay/notify"
	returnURL := paymentReturnPath("/console/log")

	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo = fmt.Sprintf("USR%dNO%s", id, tradeNo)
	totalAmount := strconv.FormatFloat(payMoney, 'f', 2, 64)

	payURL, err := buildAlipayPayURL(tradeNo, totalAmount, notifyURL, returnURL)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝 拉起支付失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		dAmount := decimal.NewFromInt(amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodAlipay,
		PaymentProvider: model.PaymentProviderAlipay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝 创建充值订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%s", id, tradeNo, req.Amount, totalAmount))
	c.JSON(http.StatusOK, gin.H{"message": "success", "url": payURL})
}

// AlipayNotify 处理支付宝异步通知。成功时必须返回纯文本 success，否则支付宝会重试。
func AlipayNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, "failure")
		return
	}
	params := make(map[string]string, len(c.Request.PostForm))
	for k := range c.Request.PostForm {
		params[k] = c.Request.PostForm.Get(k)
	}
	if !isAlipayTopUpEnabled() {
		c.String(http.StatusOK, "failure")
		return
	}

	pub, err := parseAlipayPublicKey(setting.AlipayPublicKey)
	if err != nil || verifyAlipayNotify(params, pub) != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝 webhook 验签失败 client_ip=%s", c.ClientIP()))
		c.String(http.StatusOK, "failure")
		return
	}
	if params["app_id"] != setting.AlipayAppId {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝 webhook app_id 不匹配 trade_no=%s client_ip=%s", params["out_trade_no"], c.ClientIP()))
		c.String(http.StatusOK, "failure")
		return
	}

	status := params["trade_status"]
	if status != "TRADE_SUCCESS" && status != "TRADE_FINISHED" {
		c.String(http.StatusOK, "success")
		return
	}

	tradeNo := params["out_trade_no"]
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentProvider != model.PaymentProviderAlipay {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝 回调订单不存在或网关不匹配 trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
		c.String(http.StatusOK, "failure")
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		c.String(http.StatusOK, "success")
		return
	}
	// 金额必须与下单金额一致，防止篡改
	if params["total_amount"] != strconv.FormatFloat(topUp.Money, 'f', 2, 64) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝 回调金额不一致 trade_no=%s total_amount=%s client_ip=%s", tradeNo, params["total_amount"], c.ClientIP()))
		c.String(http.StatusOK, "failure")
		return
	}

	topUp.Status = common.TopUpStatusSuccess
	if err := topUp.Update(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝 更新充值订单失败 trade_no=%s error=%q", tradeNo, err.Error()))
		c.String(http.StatusOK, "failure")
		return
	}
	dAmount := decimal.NewFromInt(int64(topUp.Amount))
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	quotaToAdd := int(dAmount.Mul(dQuotaPerUnit).IntPart())
	if err := model.IncreaseUserQuota(topUp.UserId, quotaToAdd, true); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝 更新用户额度失败 trade_no=%s user_id=%d error=%q", tradeNo, topUp.UserId, err.Error()))
		c.String(http.StatusOK, "failure")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("支付宝 充值成功 trade_no=%s user_id=%d quota_to_add=%d money=%.2f", tradeNo, topUp.UserId, quotaToAdd, topUp.Money))
	model.RecordTopupLog(topUp.UserId, fmt.Sprintf("使用支付宝充值成功，充值金额: %v，支付金额：%f", logger.LogQuota(quotaToAdd), topUp.Money), c.ClientIP(), topUp.PaymentMethod, "alipay")
	c.String(http.StatusOK, "success")
}
