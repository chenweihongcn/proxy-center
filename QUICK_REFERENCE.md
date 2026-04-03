# proxy-center 快速参考

## 🎯 三行要点

1. **是什么**: Go 编写的完整代理管理系统，支持 SOCKS5 + HTTP CONNECT，内置用户管理、流量控制、Web 控制台
2. **用来干什么**: 在 iStoreOS/NanoPi 路由器上运行，为内网设备提供代理服务，支持多用户隔离与流量控制
3. **怎么用**: 一行命令启动（Docker），自动初始化数据库，通过 Web 界面管理用户

## 🚀 最快部署 (3 分钟)

```bash
# 1. SSH 到 NanoPi
ssh root@192.168.50.94

# 2. 一键启动
sh -c "$(curl -fsSL https://raw.githubusercontent.com/chenweihongcn/proxy-center/main/deploy/istoreios-quick-start.sh)"

# 3. 打开浏览器访问
http://192.168.50.94:8090
```

**默认凭证** (首次登陆后立即修改！):
- Username: `admin`
- Password: `change-me-now`

## 📡 使用场景 (代理地址)

### SOCKS5
```
Protocol:  SOCKS5
Address:   192.168.50.94
Port:      1080
Username:  [创建的用户名]
Password:  [创建的密码]
```

### HTTP CONNECT
```
Protocol:  HTTP
Address:   192.168.50.94
Port:      8080
Username:  [创建的用户名]
Password:  [创建的密码]
```

## 📊 主要功能

| 功能 | 说明 | 访问方式 |
|------|------|----------|
| **用户管理** | 创建/编辑/删除账户 | Web UI → 用户管理 |
| **流量控制** | 按日/按月/累计限额 | Web UI 或 API |
| **到期时间** | 设置账户过期日期 | Web UI 或 API |
| **并发限制** | 限制同时连接数 | Web UI 或 API |
| **批量导入** | CSV 导入多个用户 | Web UI → 用户管理 |
| **活跃会话** | 查看/踢下在线用户 | Web UI → 会话管理 |
| **上游代理** | 配置出口代理池 | 环境变量 |
| **日志审计** | 访问域名记录 | Web UI → 日志 |

## 🔧 常用命令

### Docker 操作
```bash
# 查看服务状态
docker ps | grep proxy-center

# 查看日志 (最后 50 行，实时)
docker logs -f proxy-center --tail=50

# 重启服务
docker restart proxy-center

# 停止服务
docker stop proxy-center

# 删除容器 (数据不丢失)
docker rm proxy-center
```

### 数据库操作
```bash
# 备份数据库
docker cp proxy-center:/data/proxy-center.db ./backup.db

# 进入容器 shell
docker exec -it proxy-center sh

# 查询用户
sqlite3 /data/proxy-center.db "SELECT * FROM users;"

# 查询使用统计
sqlite3 /data/proxy-center.db "SELECT * FROM usage;"
```

## 🌐 API 快速参考

所有 API 需要 HTTP Basic Auth (管理员凭证)。

### 用户管理
```bash
# 列出所有用户
curl -u admin:password http://192.168.50.94:8090/api/users

# 创建新用户
curl -X POST http://192.168.50.94:8090/api/users \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "password": "secret",
    "role": "user",
    "max_conns": 5,
    "quota_day_mb": 500,
    "quota_month_mb": 5000,
    "quota_total_mb": 50000
  }'

# 编辑用户
curl -X PATCH http://192.168.50.94:8090/api/users/alice \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"quota_day_mb": 1000}'

# 删除用户
curl -X DELETE http://192.168.50.94:8090/api/users/alice \
  -u admin:password

# 重置使用量
curl -X POST http://192.168.50.94:8090/api/users/alice/reset-usage \
  -u admin:password

# 踢下在线用户
curl -X POST http://192.168.50.94:8090/api/users/alice/kick \
  -u admin:password
```

### CSV 批量导入
```bash
# 准备 users.csv
cat > users.csv << 'EOF'
username,password,role,enabled,expires_at,max_conns,quota_day_mb,quota_month_mb,quota_total_mb
alice,pass1,user,true,2026-12-31,5,500,5000,50000
bob,pass2,user,true,2026-12-31,3,300,3000,30000
EOF

# 导入
curl -X POST http://192.168.50.94:8090/api/users/import-csv \
  -u admin:password \
  -F "file=@users.csv"
```

## ⚙️ 环境变量

在 docker-compose.yml 或系统环境中设置：

```bash
# 必须设置
PROXY_ADMIN_PASS=your-secure-password

# 可选设置
PROXY_DB_PATH=/data/proxy-center.db
PROXY_HTTP_LISTEN=:8080
PROXY_SOCKS_LISTEN=:1080
PROXY_WEB_LISTEN=:8090
PROXY_ADMIN_USER=admin
PROXY_LOG_DOMAINS=true
PROXY_EGRESS_MODE=direct  # 或 pool
PROXY_EGRESS_POOL="http://proxy1:8080;10 http://proxy2:8080;20"
PROXY_HEALTH_TICK=10s
```

## 🔐 安全建议

1. **立即修改默认密码** — 访问 Web UI 编辑 admin 用户
2. **启用 HTTPS** — 使用 Nginx 反向代理 (可选)
3. **内网隔离** — 默认仅开放 LAN 访问，配置防火墙
4. **定期备份** — `docker cp proxy-center:/data/proxy-center.db backup.db`
5. **日志审计** — 定期查看审计日志检查异常

## ❌ 常见问题

| 问题 | 原因 | 解决 |
|------|------|------|
| Web 无法登陆 | 密码错误 | 检查 PROXY_ADMIN_PASS 或容器日志 |
| SOCKS5 连接超时 | 防火墙 / 端口未开放 | 检查 iStoreOS 防火墙规则 |
| 用户无法连接 | 用户被禁用/过期/超额 | Web UI 检查用户状态 |
| 容器持续重启 | 数据库损坏 | 删除 /data/proxy-center.db 重新初始化 |
| 内存持续增长 | 日志过多 | 清理历史日志：`DELETE FROM domain_logs WHERE created_at < datetime('now', '-30 days')` |

## 📚 详细文档

- 📖 [完整部署指南](deploy/ISTOREIOS_DEPLOYMENT.md) — 7000+ 字，覆盖所有场景
- 📋 [交付清单](DELIVERY_CHECKLIST.md) — 完整功能验收列表
- 📝 [项目主页](README.md) — API 文档 + 快速开始

## 💡 进阶用法

### 配置上游代理池
```bash
# 编辑 docker-compose.yml
environment:
  PROXY_EGRESS_MODE: pool
  PROXY_EGRESS_POOL: "http://proxy1:8080;10 socks5://proxy2:1080;20 http://proxy3:8080;5"

# 权重分配: 1:2:0.5 (比例)
# 健康检查自动故障转移
```

### OpenWrt procd 服务运行
```bash
# 创建 /etc/init.d/proxy-center
#!/bin/sh /etc/rc.common
START=99
STOP=10
USE_PROCD=1

start_service() {
  mkdir -p /opt/proxy-center/data
  procd_open_instance
  procd_set_param command /opt/proxy-center/proxyd
  procd_set_param env PROXY_DB_PATH=/opt/proxy-center/data/proxy-center.db
  procd_close_instance
}

# 启用与启动
/etc/init.d/proxy-center enable
/etc/init.d/proxy-center start
```

### 定时清理过期日志
```bash
# 在 iStoreOS 上添加 crontab
0 2 * * * sqlite3 /data/proxy-center.db "DELETE FROM domain_logs WHERE created_at < datetime('now', '-30 days');"
```

## 📞 获取帮助

- 查看项目 [Wiki](https://github.com/chenweihongcn/proxy-center/wiki)
- 提交 [Issue](https://github.com/chenweihongcn/proxy-center/issues)
- 参与 [讨论](https://github.com/chenweihongcn/proxy-center/discussions)

---

**Last Updated**: 2026-04-03
