package TritonRateLimiter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"text/template"
	"time"

	"github.com/jasinxie/TritonRateLimiter/ipchecking"
)

var (
	delay float64
)

// Config the plugin configuration.
type Config struct {
	// Prometheus Config 指标来源
	PrometheusURL string `yaml:"prometheusUrl"`
	// PromQLQuery 限流指标，比如 nv_gpu_memory_used_bytes / nv_gpu_memory_total_bytes
	PromQLQuery string `yaml:"promqlQuery"`
	// BlockThreshold 限流阈值，比如 Query 结果大于上面 PromQL 返回值
	BlockThreshold float64 `yaml:"blockThreshold"`
	// 限流策略
	BlockStrategy string `yaml:"blockStrategy"`
	// 是否开启白名单黑名单功能
	EnableIpList bool `yaml:"enableIpList"`
	// 黑名单
	Blacklist []string `yaml:"blacklist"`
	// 白名单
	Whitelist []string `yaml:"whitelist"`
	// prometheus 指标更新时间（单位秒）
	ScrapeInterval int `yaml:"scrapeInterval"`
	// 拒绝概率
	RejectProbability float64 `yaml:"rejectProbability"`
}

// for test
func init() {
	delay = 0
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
	next              http.Handler
	name              string
	template          *template.Template
	prometheusURL     string
	promQLQuery       string
	blacklist         []ipchecking.IP
	whitelist         []ipchecking.IP
	scrapeInterval    int
	blockThreshold    float64
	enableIpList      bool
	rejectProbability float64
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

	go func() {
		for {
			// 更新指标
			delay = getMetric(config.PrometheusURL, config.PromQLQuery)
			log.Printf("current delay: %v ms, scrapeInterval: %d s", delay, config.ScrapeInterval)
			time.Sleep(time.Duration(config.ScrapeInterval) * time.Second)
		}
	}()

	return &Demo{
		prometheusURL:     config.PrometheusURL,
		promQLQuery:       config.PromQLQuery,
		enableIpList:      config.EnableIpList,
		blacklist:         blacklist,
		whitelist:         whitelist,
		scrapeInterval:    config.ScrapeInterval,
		blockThreshold:    config.BlockThreshold,
		rejectProbability: config.RejectProbability,
		next:              next,
		name:              name,
		template:          template.New("demo").Delims("[[", "]]"),
	}, nil
}

func (demo *Demo) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	log.Printf("plugin triggered")

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

	// 初始化随机数生成器的种子
	rand.Seed(time.Now().UnixNano())
	// 生成一个0到1之间的随机浮点数
	randomValue := rand.Float64()
	// 以特定概率拒绝
	if delay > demo.blockThreshold && randomValue <= demo.rejectProbability {
		//rw.Header().Set("Content-Type", "application/grpc")
		rw.WriteHeader(429)
		rw.Write([]byte("the cluster is overload, please try later"))

		log.Printf("Reject!!!")
		return
	}
	demo.next.ServeHTTP(rw, req)
}

type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func getMetric(prometheusURL, promQLQuery string) float64 {
	url := fmt.Sprintf("%s/api/v1/query", prometheusURL)

	// 创建GET请求
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatalf("Error creating GET request: %v", err)
	}

	// 添加查询参数
	q := req.URL.Query()
	q.Add("query", promQLQuery)
	req.URL.RawQuery = q.Encode()

	// 添加自定义请求头
	req.Header.Set("Authorization", "")
	//req.Header.Set("User-Agent", "MyApp")

	// 发送请求
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending GET request: %v", err)
	}
	defer response.Body.Close()

	// 读取响应内容
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	// 打印响应状态码和内容
	fmt.Println("Status code:", response.StatusCode)
	fmt.Println("Response body:", string(body))

	var resp PrometheusResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("ResultType: %s\n", resp.Data.ResultType)
	var timestamp float64
	var value string
	for i, result := range resp.Data.Result {
		fmt.Printf("Result %d:\n", i+1)
		fmt.Printf("  Metric: %v\n", result.Metric)

		err = json.Unmarshal(result.Value[0], &timestamp)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(result.Value[1], &value)
		if err != nil {
			panic(err)
		}
		fmt.Printf("  Value: [%.3f, %s]\n", timestamp, value)
	}
	f, err := strconv.ParseFloat(value, 64)
	return f
}
