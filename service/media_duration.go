package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/abema/go-mp4"
)

const (
	// MaxProbedVideoURLs 单个计费参数最多探测的视频数量，超出直接拒绝请求
	MaxProbedVideoURLs = 10
	// MaxProbedVideoSeconds 单个视频/总时长的计费上限，与任务时长上限保持一致
	MaxProbedVideoSeconds = 3600

	videoProbeTimeout   = 60 * time.Second
	videoRangeBlockSize = 512 << 10
	// videoRangeMaxFetches 限制单个视频的 Range 请求次数（约 32MB），防止异常文件拖垮网关
	videoRangeMaxFetches = 64
)

var errRangeUnsupported = errors.New("range request not supported")

// ProbeVideoURLsSeconds 读取视频 URL（或 data: URL）的时长并求和，单位秒。
// 仅支持 MP4/MOV（ISO BMFF）。按秒计费只计整秒：每个视频的时长先四舍五入到整数
// （如 10.4→10、2.5→3）再求和。时长来自用户提供的文件，属于计费乘数：
// 单个与总时长都钳制在 MaxProbedVideoSeconds 以内；任何一个视频读取失败都返回错误，
// 由调用方拒绝请求，而不是按 0 秒计费。
func ProbeVideoURLsSeconds(ctx context.Context, urls []string) (float64, error) {
	if len(urls) > MaxProbedVideoURLs {
		return 0, fmt.Errorf("too many videos: %d (max %d)", len(urls), MaxProbedVideoURLs)
	}
	total := 0.0
	for i, rawURL := range urls {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}
		seconds, err := probeVideoSeconds(ctx, rawURL)
		if err != nil {
			return 0, fmt.Errorf("failed to read duration of video #%d: %w", i+1, err)
		}
		total += min(math.Round(seconds), MaxProbedVideoSeconds)
	}
	return min(total, MaxProbedVideoSeconds), nil
}

func probeVideoSeconds(ctx context.Context, rawURL string) (float64, error) {
	if strings.HasPrefix(rawURL, "data:") {
		comma := strings.Index(rawURL, ",")
		if comma < 0 || !strings.Contains(rawURL[:comma], ";base64") {
			return 0, fmt.Errorf("invalid data url")
		}
		data, err := base64.StdEncoding.DecodeString(rawURL[comma+1:])
		if err != nil {
			return 0, fmt.Errorf("invalid base64 video data: %w", err)
		}
		return readMP4Seconds(bytes.NewReader(data))
	}

	ctx, cancel := context.WithTimeout(ctx, videoProbeTimeout)
	defer cancel()

	reader := &httpRangeReader{ctx: ctx, url: rawURL, blocks: make(map[int64][]byte)}
	seconds, err := readMP4Seconds(reader)
	rangeSupported := reader.rangeSupported
	// 拿到秒数后立即释放已下载的分块，不在内存中保留任何视频数据
	reader.blocks = nil
	if err == nil || rangeSupported {
		return seconds, err
	}
	// 源站不支持 Range：整体下载到临时文件再解析（受 MaxFileDownloadMB 限制）
	return probeVideoSecondsByDownload(ctx, rawURL)
}

// readMP4Seconds 读取 moov/mvhd 中的影片总时长
func readMP4Seconds(r io.ReadSeeker) (float64, error) {
	boxes, err := mp4.ExtractBoxWithPayload(r, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeMvhd()})
	if err != nil {
		return 0, err
	}
	if len(boxes) == 0 {
		return 0, fmt.Errorf("not a mp4/mov file (mvhd box not found)")
	}
	mvhd, ok := boxes[0].Payload.(*mp4.Mvhd)
	if !ok || mvhd.Timescale == 0 {
		return 0, fmt.Errorf("invalid mvhd box")
	}
	return float64(mvhd.GetDuration()) / float64(mvhd.Timescale), nil
}

func probeVideoSecondsByDownload(ctx context.Context, rawURL string) (float64, error) {
	resp, err := doVideoProbeRequest(ctx, rawURL, "")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download video failed: status %d", resp.StatusCode)
	}

	maxMB := constant.MaxFileDownloadMB
	if maxMB <= 0 {
		maxMB = 64 // 与 MAX_FILE_DOWNLOAD_MB 默认值一致
	}
	maxBytes := int64(maxMB) << 20
	file, err := os.CreateTemp("", "video-probe-*")
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()
	written, err := io.Copy(file, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return 0, err
	}
	if written > maxBytes {
		return 0, fmt.Errorf("video exceeds %d MB and its server does not support range requests", maxMB)
	}
	return readMP4Seconds(file)
}

// doVideoProbeRequest 发起受 SSRF 防护的 GET 请求；rangeHeader 为空时下载完整文件。
// 启用 Worker 时与其他文件下载一样经由 Worker 转发。
func doVideoProbeRequest(ctx context.Context, rawURL, rangeHeader string) (*http.Response, error) {
	headers := map[string]string{}
	if rangeHeader != "" {
		headers["Range"] = rangeHeader
	}
	if system_setting.EnableWorker() {
		return DoWorkerRequest(&WorkerRequest{
			URL:     rawURL,
			Key:     system_setting.WorkerValidKey,
			Headers: headers,
		})
	}
	if err := ValidateSSRFProtectedFetchURL(rawURL); err != nil {
		return nil, fmt.Errorf("request reject: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	common.SysLog(fmt.Sprintf("probing video duration: %s", common.MaskSensitiveInfo(rawURL)))
	return GetSSRFProtectedHTTPClient().Do(req)
}

// httpRangeReader 以 HTTP Range 分块读取远程文件，实现 io.ReadSeeker，
// 让 mp4 解析只下载 box 头和 moov，而不必拉取整个视频。
type httpRangeReader struct {
	ctx            context.Context
	url            string
	size           int64
	sizeKnown      bool
	pos            int64
	blocks         map[int64][]byte
	fetches        int
	rangeSupported bool
}

func (r *httpRangeReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.sizeKnown && r.pos >= r.size {
		return 0, io.EOF
	}
	index := r.pos / videoRangeBlockSize
	block, err := r.block(index)
	if err != nil {
		return 0, err
	}
	offset := r.pos - index*videoRangeBlockSize
	if offset >= int64(len(block)) {
		return 0, io.EOF
	}
	n := copy(p, block[offset:])
	r.pos += int64(n)
	return n, nil
}

func (r *httpRangeReader) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = r.pos + offset
	case io.SeekEnd:
		if !r.sizeKnown {
			if _, err := r.block(0); err != nil {
				return 0, err
			}
		}
		next = r.size + offset
	default:
		return 0, fmt.Errorf("invalid whence %d", whence)
	}
	if next < 0 {
		return 0, fmt.Errorf("negative seek position")
	}
	r.pos = next
	return next, nil
}

func (r *httpRangeReader) block(index int64) ([]byte, error) {
	if data, ok := r.blocks[index]; ok {
		return data, nil
	}
	if r.fetches >= videoRangeMaxFetches {
		return nil, fmt.Errorf("too many range requests while reading video")
	}
	r.fetches++

	start := index * videoRangeBlockSize
	end := start + videoRangeBlockSize - 1
	resp, err := doVideoProbeRequest(r.ctx, r.url, fmt.Sprintf("bytes=%d-%d", start, end))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		r.blocks[index] = nil
		return nil, nil
	}
	if resp.StatusCode != http.StatusPartialContent {
		if resp.StatusCode == http.StatusOK {
			return nil, errRangeUnsupported
		}
		return nil, fmt.Errorf("fetch video range failed: status %d", resp.StatusCode)
	}
	r.rangeSupported = true

	// Content-Range: bytes 0-524287/12345678
	if contentRange := resp.Header.Get("Content-Range"); !r.sizeKnown && contentRange != "" {
		if slash := strings.LastIndex(contentRange, "/"); slash >= 0 {
			if total, parseErr := strconv.ParseInt(contentRange[slash+1:], 10, 64); parseErr == nil && total >= 0 {
				r.size = total
				r.sizeKnown = true
			}
		}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, videoRangeBlockSize))
	if err != nil {
		return nil, err
	}
	r.blocks[index] = data
	return data, nil
}
