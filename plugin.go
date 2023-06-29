package RequestPeekPlugin

import (
	"context"
	"encoding/json"
	"golang.org/x/time/rate"
	"io/ioutil"
	"net/http"
	"strings"
)

// Config the plugin configuration.
type Config struct {
	ReportType   string `yaml:"reportType"`
	ReportAddr   string `yaml:"reportAddr"`
	PayloadKey   string `yaml:"payloadKey"`
	InterceptCap int    `yaml:"interceptCap"`
}

// RequestPeek plugin.
type RequestPeek struct {
	next         http.Handler
	reportType   string // 上报类型
	reportAddr   string // 上报地址
	payloadKey   string // 目前至支持 json 数据
	interceptCap int    // 每秒拦截的请求数量
	limiter      *rate.Limiter
	reporter     reporter
	name         string
}

// New created r new RequestPeek plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	rateCount := rate.Limit(config.InterceptCap)
	rst := &RequestPeek{
		reportType: config.ReportType,
		reportAddr: config.ReportAddr,
		limiter:    rate.NewLimiter(rateCount, int(rateCount*2)),
		// 暂时只支持 udp
		reporter: newUdpReporter(config.ReportAddr),
		next:     next,
		name:     name,
	}

	return rst, nil
}

func (r *RequestPeek) canPeek() bool {
	if r.limiter.Allow() {
		return true
	}
	return false
}

func (r *RequestPeek) intercept(req *http.Request) []byte {
	if !strings.Contains(strings.ToLower(req.Header.Get("Content-Type")), "json") {
		return nil
	}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil
	}
	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil
	}
	value, ok := data[r.payloadKey]
	if !ok {
		return nil
	}
	stringValue, ok := value.(string)
	if !ok {
		return nil
	}
	return []byte(stringValue)
}

func (r *RequestPeek) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if r.canPeek() {
		target := r.intercept(req)
		go func(payload []byte) {
			if payload == nil {
				return
			}
			r.limiter.Wait(context.Background())
			r.reporter.Report(payload)
		}(target)
	}
	r.next.ServeHTTP(rw, req)
}
