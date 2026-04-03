# iStoreOS 部署指南

## 环境信息

- **设备**: FriendlyElec NanoPi R2S Plus
- **系统**: iStoreOS 24.10.5
- **架构**: ARMv8 (linux/arm64)
- **内核**: 6.6.119
- **预装**: Docker, Go (1.21+), SQLite3

## 部署方案

有三种推荐的部署方式：

### 方案 1: Docker 容器部署（推荐）

#### 1.1 在 iStoreOS 上直接拉取镜像

```bash
# SSH 到 iStoreOS
ssh root@192.168.50.94

# 创建数据目录
mkdir -p /opt/proxy-center/data
cd /opt/proxy-center

# 使用官方镜像运行（如果已发布到 registry）
docker run -d \
  --name proxy-center \
  --restart always \
  -p 8080:8080 \
  -p 1080:1080 \
  -p 8090:8090 \
  -v /opt/proxy-center/data:/data \
  --env PROXY_DB_PATH=/data/proxy-center.db \
  --env PROXY_ADMIN_USER=admin \
  --env PROXY_ADMIN_PASS=your-secure-password \
  proxy-center:armv8-latest
```

#### 1.2 从源码构建 armv8 镜像

**在 Windows 主机上跨平台编译**:

```powershell
# 方法 A: 使用 docker buildx（推荐）
docker buildx create --use
docker buildx build --platform linux/arm64 \
  -t proxy-center:armv8-1.0 \
  --push .

# 方法 B: 使用远程 iStoreOS 编译
.\build-armv8.ps1 -iStoreOSIP 192.168.50.94 -Version 1.0
# 这会输出 build/proxyd-arm64 二进制

# 然后在 iStoreOS 上手动构建镜像
scp build/proxyd-arm64 root@192.168.50.94:/tmp/
ssh root@192.168.50.94 << 'EOF'
  docker build -t proxy-center:armv8-1.0 - << 'DOCKER'
    FROM alpine:3.21
    RUN adduser -D -H -u 10001 proxy
    WORKDIR /app
    COPY /tmp/proxyd-arm64 /usr/local/bin/proxyd
    RUN chmod +x /usr/local/bin/proxyd && mkdir -p /data && chown proxy:proxy /data
    USER proxy
    EXPOSE 8080 1080 8090
    ENTRYPOINT ["proxyd"]
  DOCKER
EOF
```

#### 1.3 编写 compose 文件在 iStoreOS 上运行

在 iStoreOS 上创建 `/opt/proxy-center/docker-compose.yml`:

```yaml
version: '3.8'
services:
  proxy-center:
    image: proxy-center:armv8-1.0
    container_name: proxy-center
    restart: always
    ports:
      - "8080:8080"   # HTTP CONNECT
      - "1080:1080"   # SOCKS5
      - "8090:8090"   # Web 管理界面
    environment:
      PROXY_DB_PATH: /data/proxy-center.db
      PROXY_HTTP_LISTEN: :8080
      PROXY_SOCKS_LISTEN: :1080
      PROXY_WEB_LISTEN: :8090
      PROXY_ADMIN_USER: admin
      PROXY_ADMIN_PASS: your-secure-password
      PROXY_LOG_DOMAINS: "true"
      # 上游代理配置（可选）
      # PROXY_EGRESS_MODE: pool
      # PROXY_EGRESS_POOL: "http://proxy1:8080;10 http://proxy2:8080;20"
    volumes:
      - ./data:/data
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "-", "http://localhost:8090/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "3"
```

启动服务:
```bash
cd /opt/proxy-center
docker-compose up -d

# 查看日志
docker-compose logs -f proxy-center

# 停止服务
docker-compose down
```

### 方案 2: 直接编译运行（轻量化）

适用于已在 iStoreOS 上安装 Go 的情况。

```bash
ssh root@192.168.50.94

cd /opt/proxy-center
git clone https://github.com/chenweihongcn/proxy-center.git
cd proxy-center

# 编译
CGO_ENABLED=0 go build -o proxyd ./cmd/proxyd

# 运行
./proxyd
  # 环境变量配置（可选）
  # export PROXY_DB_PATH=/opt/proxy-center/proxy-center.db
  # export PROXY_ADMIN_PASS=your-password
```

### 方案 3: OpenWrt procd 服务启动（生产推荐）

在 iStoreOS/OpenWrt 上创建 `/etc/init.d/proxy-center`:

```sh
#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1

start_service() {
  mkdir -p /opt/proxy-center/data
  procd_open_instance
  procd_set_param command /opt/proxy-center/proxyd
  procd_set_param env \
    PROXY_DB_PATH=/opt/proxy-center/data/proxy-center.db \
    PROXY_HTTP_LISTEN=:8080 \
    PROXY_SOCKS_LISTEN=:1080 \
    PROXY_WEB_LISTEN=:8090 \
    PROXY_ADMIN_USER=admin \
    PROXY_ADMIN_PASS=your-secure-password \
    PROXY_LOG_DOMAINS=true
  procd_set_param respawn 3600 5 0
  procd_set_param stdout 1
  procd_set_param stderr 1
  procd_close_instance
}
```

启用和启动：
```bash
chmod +x /etc/init.d/proxy-center
/etc/init.d/proxy-center enable
/etc/init.d/proxy-center start
/etc/init.d/proxy-center status

# 查看日志
logread -f | grep proxy-center
```

## 配置管理

### 环境变量清单

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `PROXY_DB_PATH` | `/data/proxy-center.db` | SQLite 数据库文件路径 |
| `PROXY_HTTP_LISTEN` | `:8080` | HTTP CONNECT 监听地址 |
| `PROXY_SOCKS_LISTEN` | `:1080` | SOCKS5 监听地址 |
| `PROXY_WEB_LISTEN` | `:8090` | Web 管理界面地址 |
| `PROXY_ADMIN_USER` | `admin` | Web 管理员用户名 |
| `PROXY_ADMIN_PASS` | `change-me-now` | Web 管理员密码（**强烈建议修改**） |
| `PROXY_LOG_DOMAINS` | `true` | 是否记录访问域名到数据库 |
| `PROXY_EGRESS_MODE` | `direct` | 出口模式: `direct` 或 `pool` |
| `PROXY_EGRESS_POOL` | 空 | 上游代理池格式: `url;weight url;weight ...` |
| `PROXY_HEALTH_TICK` | `10s` | 健康检查间隔 |

### 上游代理池配置示例

```bash
export PROXY_EGRESS_MODE=pool
export PROXY_EGRESS_POOL="http://proxy1.example.com:8080;10 http://proxy2.example.com:8080;20 socks5://proxy3.example.com:1080;5"

# 权重分配：proxy1 占 10/(10+20+5)=⅓, proxy2 占 ⅗, proxy3 占 1/15
```

## 初始化与管理

### 1. 首次启动

```bash
# 运行后，代理中心会自动创建数据库表并插入默认管理员
curl -X GET http://192.168.50.94:8090/healthz
# 应返回 {"status":"ok"}
```

### 2. 修改管理员密码

访问 Web 界面: `http://192.168.50.94:8090`

- 使用默认凭证登录 (`admin` / `change-me-now`)
- 点击"用户管理" → 编辑 `admin` 用户 → 修改密码

### 3. 创建代理账户

通过 Web 界面或 API:

```bash
curl -X POST http://192.168.50.94:8090/api/users \
  -H "Authorization: Basic $(echo -n 'admin:your-password' | base64)" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user1",
    "password": "secure-pass",
    "role": "user",
    "max_conns": 5,
    "quota_day_mb": 1000,
    "quota_month_mb": 10000,
    "quota_total_mb": 100000
  }'
```

### 4. CSV 批量导入

准备 CSV 文件 (`users.csv`):

```csv
username,password,role,enabled,expires_at,max_conns,quota_day_mb,quota_month_mb,quota_total_mb
user1,pass1,user,true,2026-12-31,5,500,5000,50000
user2,pass2,user,true,2026-12-31,3,300,3000,30000
user3,pass3,operator,true,,10,1000,10000,100000
```

导入：

```bash
curl -X POST http://192.168.50.94:8090/api/users/import-csv \
  -H "Authorization: Basic $(echo -n 'admin:password' | base64)" \
  -F "file=@users.csv"
```

## 监控与维护

### 性能监控

iStoreOS 内置的 LuCI 界面可查看：
- 内存使用: `Status → System → Memory`
- 磁盘空间: `Status → System → Storage space usage`
- 网络流量: `Status → System → eth0 / eth1`

### 日志管理

```bash
# 查看最近日志（Docker）
docker-compose logs --tail=100 proxy-center

# 或查看 OpenWrt procd 日志
logread -e proxy-center | tail -100

# 日志保留策略
# - 域名日志自动在 30 天后清理
# - 审计日志保留 90 天
```

### 数据备份

```bash
# 备份数据库
docker cp proxy-center:/data/proxy-center.db ./proxy-center.db.backup

# 或
scp root@192.168.50.94:/data/proxy-center.db ./proxy-center.db.backup

# 恢复
docker cp ./proxy-center.db.backup proxy-center:/data/proxy-center.db
```

### 常见问题排查

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| Web 无法登陆 | 密码错误或服务未启动 | 检查 `PROXY_ADMIN_PASS` 环境变量；`docker logs proxy-center` |
| SOCKS5 连接超时 | 可能是防火墙或端口未开放 | `ss -tuln \| grep 1080`; 检查 iStoreOS 防火墙规则 |
| CPU 占用高 | 健康检查频率过高或并发连接多 | 降低 `PROXY_HEALTH_TICK`；监控 `PROXY_MAX_CONNS` |
| 磁盘空间满 | 日志或数据库文件过大 | 手动清理日志；使用 `VACUUM` 优化 SQLite |

## 升级与回滚

### 升级到新版本

```bash
# 备份当前数据
docker-compose down
cp -r data data.backup

# 更新镜像（假设新版本已发布）
docker pull proxy-center:armv8-1.1

# 更新 compose 文件中的镜像版本
# 修改: image: proxy-center:armv8-1.1

# 重启
docker-compose up -d

# 验证
docker-compose logs proxy-center
```

### 遇到问题时回滚

```bash
# 停止服务
docker-compose down

# 恢复数据
rm -rf data
cp -r data.backup data

# 回退镜像版本
docker image rm proxy-center:armv8-1.1
docker pull proxy-center:armv8-1.0

# 重启
docker-compose up -d
```

## 网络配置

### iStoreOS 防火墙规则

若需要从外网访问代理，需在 iStoreOS LuCI 中配置端口转发：

1. **开放 WAN 到 LAN 的转发**:
   - 登陆 LuCI: `http://192.168.50.1`
   - 进入 `Network → Firewall → Port Forwards`
   - 添加规则:
     ```
     Protocol: TCP/UDP
     External Zone: wan
     Internal Zone: lan
     External Port: 8080
     Internal Address: 192.168.x.x (proxy-center IP)
     Internal Port: 8080
     ```

2. **允许入站连接**:
   - `Network → Firewall → Traffic Rules`
   - 添加允许 8080, 1080, 8090 的入站规则

### 内网隔离（推荐）

默认配置仅允许 LAN 内访问。若需要限制进一步，可在容器配置中：

```yaml
services:
  proxy-center:
    ports:
      # 只绑定 LAN 接口 (eth1, br-lan)
      - "192.168.1.1:8080:8080"   # LAN only
      - "192.168.1.1:1080:1080"
      - "192.168.1.1:8090:8090"
```

## 性能调优

### 并发连接优化

对于高并发场景（1000+ 并发用户），调整：

```bash
# 在 OpenWrt procd 脚本或 docker-compose 中
Environment="GOMAXPROCS=2"      # 限制 Go 运行时线程数
# 或通过环境变量在容器中设置
```

### 数据库优化

定期执行维护任务：

```bash
# SSH 到 iStoreOS
sqlite3 /data/proxy-center.db << 'SQL'
  -- 清理过期的域名日志（超过 30 天）
  DELETE FROM domain_logs WHERE created_at < datetime('now', '-30 days');
  
  -- 清理已删除用户的使用记录
  DELETE FROM usage WHERE user_id NOT IN (SELECT id FROM users);
  
  -- 优化数据库
  VACUUM;
  ANALYZE;
SQL
```

### 上游代理优化

```bash
# 提高健康检查灵敏度
export PROXY_HEALTH_TICK=5s

# 调整上游池权重（基于实际网络特性）
export PROXY_EGRESS_POOL="http://fast-proxy:8080;50 http://backup-proxy:8080;10"
```

## 故障排查检查清单

- [ ] 容器是否正常运行: `docker ps | grep proxy-center`
- [ ] 端口是否已监听: `netstat -tuln | grep -E '8080|1080|8090'` (或 iStoreOS LuCI)
- [ ] 数据库文件是否存在: `ls -la /data/proxy-center.db`
- [ ] 管理员凭证是否正确: `echo $PROXY_ADMIN_PASS`
- [ ] 网络连通性: `ping 192.168.50.94 -c 1` (从 LAN 内)
- [ ] 防火墙规则: LuCI 中 `Network → Firewall` 检查是否有阻止规则

## 支持与反馈

- 项目主页: https://github.com/chenweihongcn/proxy-center
- 问题反馈: https://github.com/chenweihongcn/proxy-center/issues
- 文档: https://github.com/chenweihongcn/proxy-center/wiki

---

最后更新: 2026-04-03
