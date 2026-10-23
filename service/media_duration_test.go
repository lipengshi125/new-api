package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mp4Box 生成一个 ISO BMFF box：4 字节大小 + 4 字节类型 + payload
func mp4Box(boxType string, payload []byte) []byte {
	out := make([]byte, 8, 8+len(payload))
	binary.BigEndian.PutUint32(out, uint32(8+len(payload)))
	copy(out[4:], boxType)
	return append(out, payload...)
}

// buildTestMP4 构造最小可解析 MP4：ftyp + mdat(填充) + moov/mvhd(v0)，
// moov 放在文件末尾，覆盖非 faststart 视频需要跨块读取的场景。
func buildTestMP4(seconds float64, mdatSize int) []byte {
	const timescale = 1000
	mvhd := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhd[12:], timescale)
	binary.BigEndian.PutUint32(mvhd[16:], uint32(seconds*timescale))
	binary.BigEndian.PutUint32(mvhd[20:], 0x00010000) // rate
	binary.BigEndian.PutUint16(mvhd[24:], 0x0100)     // volume
	binary.BigEndian.PutUint32(mvhd[96:], 2)          // next track id

	ftyp := mp4Box("ftyp", append([]byte("isom"), 0, 0, 2, 0, 'i', 's', 'o', 'm', 'm', 'p', '4', '1'))
	mdat := mp4Box("mdat", make([]byte, mdatSize))
	moov := mp4Box("moov", mp4Box("mvhd", mvhd))
	return bytes.Join([][]byte{ftyp, mdat, moov}, nil)
}

func allowLocalVideoProbe(t *testing.T) {
	t.Helper()
	fetchSetting := system_setting.GetFetchSetting()
	original := *fetchSetting
	t.Cleanup(func() { *fetchSetting = original })
	// 测试服务器在 127.0.0.1，关闭 SSRF 防护以便访问本地地址
	fetchSetting.EnableSSRFProtection = false
	InitHttpClient()
}

func newVideoServer(t *testing.T, files map[string][]byte, supportRange bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if supportRange {
			http.ServeContent(w, r, "video.mp4", time.Time{}, bytes.NewReader(data))
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestProbeVideoURLsSeconds(t *testing.T) {
	allowLocalVideoProbe(t)

	// mdat 大于一个 Range 块，迫使解析器跨块 seek 到末尾的 moov
	files := map[string][]byte{
		"/a.mp4":    buildTestMP4(10, videoRangeBlockSize+1024),
		"/b.mp4":    buildTestMP4(2.5, 100),
		"/down.mp4": buildTestMP4(10.4, 100),
		"/up.mp4":   buildTestMP4(10.6, 100),
		"/huge.mp4": buildTestMP4(4000, 100),
		"/bad.mp4":  []byte("this is not a video"),
	}

	for _, supportRange := range []bool{true, false} {
		server := newVideoServer(t, files, supportRange)
		name := "range"
		if !supportRange {
			name = "full download"
		}

		t.Run(name+"/sums whole-second durations", func(t *testing.T) {
			// 10s + 2.5s：每个视频先四舍五入到整秒（2.5→3），按整秒计费
			seconds, err := ProbeVideoURLsSeconds(context.Background(), []string{server.URL + "/a.mp4", server.URL + "/b.mp4"})
			require.NoError(t, err)
			assert.Equal(t, 13.0, seconds)
		})

		t.Run(name+"/rounds each video to whole seconds", func(t *testing.T) {
			// 10.4s→10，10.6s→11：舍入作用于每个视频，不保留小数
			seconds, err := ProbeVideoURLsSeconds(context.Background(), []string{server.URL + "/down.mp4", server.URL + "/up.mp4"})
			require.NoError(t, err)
			assert.Equal(t, 21.0, seconds)
		})

		t.Run(name+"/clamps to max", func(t *testing.T) {
			seconds, err := ProbeVideoURLsSeconds(context.Background(), []string{server.URL + "/huge.mp4", server.URL + "/a.mp4"})
			require.NoError(t, err)
			assert.Equal(t, float64(MaxProbedVideoSeconds), seconds)
		})

		t.Run(name+"/unreadable video rejects", func(t *testing.T) {
			_, err := ProbeVideoURLsSeconds(context.Background(), []string{server.URL + "/a.mp4", server.URL + "/bad.mp4"})
			require.Error(t, err, "an unreadable video must reject the request instead of billing it as 0s")
		})

		t.Run(name+"/missing video rejects", func(t *testing.T) {
			_, err := ProbeVideoURLsSeconds(context.Background(), []string{server.URL + "/missing.mp4"})
			require.Error(t, err)
		})
	}
}

func TestProbeVideoURLsSecondsDataURLAndLimits(t *testing.T) {
	dataURL := "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(buildTestMP4(3, 10))
	seconds, err := ProbeVideoURLsSeconds(context.Background(), []string{dataURL})
	require.NoError(t, err)
	assert.InDelta(t, 3.0, seconds, 1e-9)

	tooMany := make([]string, MaxProbedVideoURLs+1)
	for i := range tooMany {
		tooMany[i] = dataURL
	}
	_, err = ProbeVideoURLsSeconds(context.Background(), tooMany)
	require.Error(t, err)
}

func TestProbeVideoURLsSecondsRespectsSSRFProtection(t *testing.T) {
	fetchSetting := system_setting.GetFetchSetting()
	original := *fetchSetting
	t.Cleanup(func() { *fetchSetting = original })
	fetchSetting.EnableSSRFProtection = true
	fetchSetting.AllowPrivateIp = false
	InitHttpClient()

	server := newVideoServer(t, map[string][]byte{"/a.mp4": buildTestMP4(10, 100)}, true)
	_, err := ProbeVideoURLsSeconds(context.Background(), []string{server.URL + "/a.mp4"})
	require.Error(t, err, "private network video URLs must be blocked by SSRF protection")
}
