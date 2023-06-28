用于对Triton服务进行限流的Traefik插件

cloned from [plugindemo](https://github.com/traefik/plugindemo)

## 使用方式
traefik配置文件 traefik.sample.yml 示例如下：
```yaml
################################################################
#
# Configuration sample for Traefik v2.
#
# For Traefik v1: https://github.com/traefik/traefik/blob/v1.7/traefik.sample.toml
#
################################################################

################################################################
# Global configuration
################################################################
global:
  checkNewVersion: true
  sendAnonymousUsage: true

################################################################
# EntryPoints configuration
################################################################

# EntryPoints definition
#
# Optional
#
entryPoints:
  web:
    address: :80

  websecure:
    address: :443

  http:
    address: :8089
  
  grpc:
    address: :50053

################################################################
# Traefik logs configuration
################################################################

# Traefik logs
# Enabled by default and log to stdout
#
# Optional
#
log:
  # Log level
  #
  # Optional
  # Default: "ERROR"
  #
 level: INFO

  # Sets the filepath for the traefik log. If not specified, stdout will be used.
  # Intermediate directories are created if necessary.
  #
  # Optional
  # Default: os.Stdout
  #
#  filePath: log/traefik.log

  # Format is either "json" or "common".
  #
  # Optional
  # Default: "common"
  #
#  format: json

################################################################
# Access logs configuration
################################################################

# Enable access logs
# By default it will write to stdout and produce logs in the textual
# Common Log Format (CLF), extended with additional fields.
#
# Optional
#
accessLog: {}
  # Sets the file path for the access log. If not specified, stdout will be used.
  # Intermediate directories are created if necessary.
  #
  # Optional
  # Default: os.Stdout
  #
#  filePath: /path/to/log/log.txt

  # Format is either "json" or "common".
  #
  # Optional
  # Default: "common"
  #
#  format: json

################################################################
# API and dashboard configuration
################################################################

# Enable API and dashboard
#
# Optional
#
api:
  # Enable the API in insecure mode
  #
  # Optional
  # Default: false
  #
 insecure: true

  # Enabled Dashboard
  #
  # Optional
  # Default: true
  #
 dashboard: true

################################################################
# Ping configuration
################################################################

# Enable ping
#ping:
  # Name of the related entry point
  #
  # Optional
  # Default: "traefik"
  #
#  entryPoint: traefik

################################################################
# Docker configuration backend
################################################################

providers:
  file:
    filename: "dynamic.yaml"
  # Enable Docker configuration backend
#  docker:
    # Docker server endpoint. Can be a tcp or a unix socket endpoint.
    #
    # Required
    # Default: "unix:///var/run/docker.sock"
    #
#    endpoint: tcp://10.10.10.10:2375

    # Default host rule.
    #
    # Optional
    # Default: "Host(`{{ normalize .Name }}`)"
    #
#    defaultRule: Host(`{{ normalize .Name }}.docker.localhost`)

    # Expose containers by default in traefik
    #
    # Optional
    # Default: true
    #
#    exposedByDefault: false

experimental:
  localPlugins:
    tritonRateLimiter:
      moduleName: github.com/jasinxie/TritonRateLimiter
```

资源文件 dynamic.yaml 如下：
```yaml
http:
  middlewares:
    tritonRateLimiter:
      plugin:
        tritonRateLimiter:
            enableIpList: false
            whitelist:
              - "::1"
              - "127.0.0.1"
            blacklist:
              - "192.168.0.0/24"
            scrapeInterval: 5
            blockStrategy: ""
            blockThreshold: 1000
            promqlQuery: "round(sum (increase(nv_inference_request_duration_us[24h])) / sum (increase(nv_inference_request_success[24h])) / 1000)"
            prometheusUrl: "http://81.69.152.80:9090"
            rejectProbability: 0.5
          
  routers:
    http-router:
      entryPoints:
        - http
      rule: "PathPrefix(`/`)"
      service: http-service
      middlewares:
        - tritonRateLimiter

    grpc-router:
      entryPoints:
        - grpc
      rule: "PathPrefix(`/`)"
      service: grpc-service
      middlewares:
        - tritonRateLimiter

  services:
    http-service:
      loadBalancer:
        servers:
          - url: "http://127.0.0.1:8088"

    grpc-service:
      loadBalancer:
        servers:
          - url: "h2c://127.0.0.1:50052"
```

采用 [Local Mode](https://plugins.traefik.io/install) 目录结构如下：
```bash
(base)  ~/Downloads/traefik_v2.10.1_darwin_arm64/ tree
.
├── CHANGELOG.md
├── LICENSE.md
├── dynamic.yaml
├── plugins-local
│   └── src
│       └── github.com
│           └── jasinxie
│               └── TritonRateLimiter
│                   ├── LICENSE
│                   ├── Makefile
│                   ├── demo.go
│                   ├── demo_test.go
│                   ├── go.mod
│                   ├── go.sum
│                   ├── ipchecking
│                   │   ├── ipChecking.go
│                   │   └── ipChecking_test.go
│                   └── readme.md
├── plugins-storage
│   ├── archives
│   └── sources
│       └── gop-1576004276
├── traefik
└── traefik.sample.yml

10 directories, 14 files
```