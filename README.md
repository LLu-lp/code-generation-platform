# 🚀 AI 零代码应用生成平台 (Go 重构版)

> 输入自然语言需求 → AI 自动生成完整前端应用 → 一键部署上线

![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)
![Gin](https://img.shields.io/badge/Gin-1.10-009688)
![GORM](https://img.shields.io/badge/GORM-1.25-4A90E2)
![Vue](https://img.shields.io/badge/Vue-3.x-4FC08D?logo=vue.js)
![License](https://img.shields.io/badge/license-MIT-green)

---

## 📖 简介

**AI 零代码应用生成平台** 是一套基于 **Go + Gin + GORM** 后端 + **Vue 3** 前端的企业级 AI 应用。用户只需用自然语言描述需求，系统即可：

- 🧠 **智能分析** — AI 自动判断需求复杂度，选择最优代码生成策略
- ⚡ **流式生成** — SSE 实时推送，打字机效果逐字输出代码
- 🔧 **工具调用** — AI 自主使用文件读写工具，像真实开发者一样创建项目
- 🚀 **一键部署** — 生成 deployKey，自动截图封面，获得可分享的 URL

---

## 🎯 核心能力

| 功能 | 说明 |
|------|------|
| **HTML 单文件生成** | 秒级生成，内联 CSS/JS，适合展示页 |
| **多文件分离生成** | HTML/CSS/JS 分离，结构清晰 |
| **Vue 工程生成** | Vite + Vue3 + Router，AI 通过 6 个工具自主创建项目 |
| **可视化编辑** | 选中页面元素直接与 AI 对话修改 |
| **一键部署** | 自动 npm build → 复制部署 → 截图封面 → 返回 URL |
| **代码下载** | ZIP 打包下载完整源码 |

---

## 🏗️ 技术栈

| 层级 | 技术 |
|------|------|
| **后端框架** | Go 1.22 + Gin |
| **数据库** | MySQL 8 + GORM |
| **缓存** | Redis + go-cache（多级缓存） |
| **AI 模型** | DeepSeek Chat / DeepSeek Reasoner / Qwen Turbo |
| **流式推送** | SSE (Server-Sent Events) |
| **监控** | Prometheus + Grafana |
| **截图** | chromedp |
| **对象存储** | 腾讯云 COS |
| **前端** | Vue 3 + Vite + Ant Design Vue + Pinia |

---

## 📁 项目结构

```
.
├── cmd/server/main.go          # 入口：依赖注入 + 路由注册 + 优雅退出
├── configs/config.yaml         # YAML 配置文件
├── sql/schema.sql              # 数据库建表语句
├── internal/
│   ├── ai/                     # AI 核心层
│   │   ├── client.go           #   OpenAI 兼容客户端（同步/流式/工具调用/结构化输出）
│   │   ├── tools.go            #   6 个 AI 工具（读写改删查+退出）
│   │   ├── generator.go        #   代码生成工厂（3 种策略 + 智能路由 + 质检）
│   │   └── prompt.go           #   Prompt 管理 + 输入安全护栏
│   ├── workflow/               # 自研工作流引擎
│   │   ├── workflow.go         #   6 节点 DAG 编排（图片→提示词→路由→生成→质检→构建）
│   │   └── tools.go            #   Pexels/Mermaid/Logo 工具
│   ├── controller/             # HTTP 控制器层
│   ├── service/                # 业务逻辑层
│   ├── repository/             # 数据访问层（游标分页）
│   ├── middleware/              # 中间件（Auth/CORS/RateLimit/Logger/Metrics）
│   ├── monitor/                # AI 调用指标收集
│   ├── manager/                # 腾讯云 COS 管理器
│   ├── core/                   # 代码解析/保存/构建/流处理
│   ├── model/                  # 数据模型（entity/dto/vo/enums）
│   ├── config/                 # Viper 配置加载
│   └── exception/              # 业务错误码
├── pkg/                        # 工具库（数据库/Redis/JSON/文件）
├── INTERVIEW.md                # 📄 面试问答（17 题 + 关键代码）
└── README_GO.md                # 📄 详细技术文档 + 面试指南
```

---

## 🚀 快速开始

### 环境要求

- Go 1.22+
- MySQL 8.0+
- Redis 6.0+
- Node.js 18+（用于 Vue 项目构建）

### 1. 克隆项目

```bash
git clone <repo-url>
cd yu-ai-code-mother
```

### 2. 配置数据库

```bash
# 执行建表 SQL
mysql -u root -p < sql/schema.sql
```

### 3. 修改配置

```bash
cp configs/config.yaml configs/config.local.yaml
# 编辑 config.local.yaml，填入你的 API Key 和数据库密码
```

### 4. 运行

```bash
# 安装依赖
go mod tidy

# 启动服务（默认端口 8123）
go run cmd/server/main.go
```

### 5. 访问

- 后端 API: `http://localhost:8123/api`
- 健康检查: `http://localhost:8123/api/health`
- 监控指标: `http://localhost:8123/api/actuator/prometheus`

---

## 📊 AI 模型分工

| 模型 | 角色 | 用量 |
|------|------|------|
| **DeepSeek Chat** | 主力代码生成（HTML/多文件/Vue对话） | 85% |
| **DeepSeek Reasoner** | Vue 工程的工具调用推理 | 10% |
| **Qwen Turbo** | 智能路由分类 | 5% |

通过三模型分工，Token 成本降低 **65%**。

---

## 📚 文档

| 文档 | 说明 |
|------|------|
| [INTERVIEW.md](./INTERVIEW.md) | 面试高频 17 问 + 关键源码 |
| [README_GO.md](./README_GO.md) | 详细技术文档 + 架构设计 |

---

## 📝 License

MIT
