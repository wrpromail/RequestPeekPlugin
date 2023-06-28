package TritonRateLimiter

import (
	"context"
	"errors"
	"fmt"
	"github.com/jasinxie/TritonRateLimiter/ipchecking"
	"github.com/prometheus/common/model"
	"log"
	"net"
	"net/http"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

var (
	delay float64
)

// Config the plugin configuration.
type Config struct {
	// Prometheus Config 指标来源
	PrometheusURL string `yaml:"prometheus_url,omitempty"`
	// PromQLQuery 限流指标，比如 nv_gpu_memory_used_bytes / nv_gpu_memory_total_bytes
	PromQLQuery string `yaml:"promql_query,omitempty"`
	// BlockThreshold 限流阈值，比如 Query 结果大于上面 PromQL 返回值
	BlockThreshold float64 `yaml:"block_threshold,omitempty"`
	// 限流策略
	BlockStrategy string `yaml:"block_strategy,omitempty"`
	// 是否开启白名单黑名单功能
	EnableIpList bool `yaml:"enable_ip_list"`
	// 黑名单
	Blacklist []string `yaml:"blacklist"`
	// 白名单
	Whitelist []string `yaml:"whitelist"`
	// prometheus 指标更新时间（单位秒）
	ScrapeInterval int `yaml:"scrape_interval"`
}

// for test
func init() {
	delay = 10000
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		PrometheusURL: "http://81.69.152.80:9090",
		// 平均每个请求处理时间(ms)
		PromQLQuery: "round(sum (increase(nv_inference_request_duration_us[24h])) / sum (increase(nv_inference_request_success[24h])) / 1000)",
		// 每个请求处理时间超过5s，直接拒绝
		BlockThreshold: 5000.0,
		BlockStrategy:  "",
		Blacklist:      nil,
		Whitelist:      nil,
		EnableIpList:   false,
	}
}

// Demo demo Demo plugin.
type Demo struct {
	next           http.Handler
	name           string
	template       *template.Template
	prometheusURL  string
	promQLQuery    string
	blacklist      []ipchecking.IP
	whitelist      []ipchecking.IP
	scrapeInterval int
	blockThreshold float64
	enableIpList   bool
}

// New created demo new Demo plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.PrometheusURL == "" {
		return nil, errors.New("prometheus url is not set")
	}

	whitelist, err := ipchecking.StrToIP(config.Whitelist)
	if err != nil {
		return nil, err
	}

	blacklist, err := ipchecking.StrToIP(config.Blacklist) // Do not mistake with Black Eyed Peas
	if err != nil {
		return nil, err
	}
	for _, ip := range blacklist {
		log.Printf("Blacklisted: '%s'", ip.ToString())
	}

	return &Demo{
		prometheusURL:  config.PrometheusURL,
		promQLQuery:    config.PromQLQuery,
		enableIpList:   config.EnableIpList,
		blacklist:      blacklist,
		whitelist:      whitelist,
		scrapeInterval: config.ScrapeInterval,
		blockThreshold: config.BlockThreshold,
		next:           next,
		name:           name,
		template:       template.New("demo").Delims("[[", "]]"),
	}, nil
}

func (demo *Demo) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	go func() {
		for {
			// 更新指标
			//delay = demo.getMetric()
			time.Sleep(time.Duration(demo.scrapeInterval) * time.Second)
		}
	}()

	if demo.enableIpList {
		remoteIP, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			log.Println(remoteIP + " is not a valid IP or a IP/NET")
			return
		}

		// Blacklist
		for _, ip := range demo.blacklist {
			if ip.CheckIPInSubnet(remoteIP) {
				log.Println(remoteIP + " is blacklisted")
				rw.WriteHeader(http.StatusForbidden)
				return
			}
		}
		// Whitelist
		for _, ip := range demo.whitelist {
			if ip.CheckIPInSubnet(remoteIP) {
				log.Println(remoteIP + " is whitelisted")
				demo.next.ServeHTTP(rw, req)
				return
			}
		}
	}

	if delay > demo.blockThreshold {
		rw.Header().Set("Content-Type", "application/grpc")
		rw.WriteHeader(429)
		return
	}
	demo.next.ServeHTTP(rw, req)
}

func (demo *Demo) getMetric() float64 {
	// 创建Prometheus API客户端
	client, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("%s/query", demo.prometheusURL),
	})
	if err != nil {
		log.Fatalf("Error creating Prometheus client: %v", err)
	}

	// 创建v1 API接口实例
	api := v1.NewAPI(client)

	// 查询Prometheus指标
	result, warnings, err := api.Query(context.Background(), demo.promQLQuery, time.Now())
	if err != nil {
		log.Fatalf("Error querying Prometheus: %v", err)
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}

	// 打印查询结果
	//fmt.Printf("Query result: %v\n", result)

	vector, ok := result.(model.Vector)
	if !ok {
		log.Fatalf("Unexpected result type: %T", result)
	}

	var latestValue float64
	for _, sample := range vector {
		fmt.Printf("Timestamp: %v, Value: %v\n", sample.Timestamp, sample.Value)
		latestValue = float64(sample.Value)
	}

	return latestValue
}
