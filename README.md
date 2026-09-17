# 考古发掘出土文物编目系统（DigCatalog）

面向考古工地出土文物登记与编目的全栈演示项目：支持发掘工地、探方/发掘单位、出土文物、材质字典的 CRUD，以及概览统计。

## 技术栈

- **前端**: Vue 3 + Vite + Pinia + Vue Router（Composition API + `<script setup>`）
- **后端**: Go 1.21+ + Gin + GORM
- **数据库**: MySQL 8.0
- **认证**: JWT + bcrypt

## 一键启动

```bash
docker compose up --build
```

启动完成后访问：

| 服务 | 地址 |
|------|------|
| 前端 | http://localhost:3200 |
| 后端 API | http://localhost:8200/api |
| MySQL | localhost:3307（用户 `root` / 密码 `root`，库名 `digcatalog`） |

停止服务：

```bash
docker compose down
```

清除数据卷后重建：

```bash
docker compose down -v
docker compose up --build
```

## 测试账号

| 用户名 | 密码 | 角色 |
|--------|------|------|
| `admin` | `123456` | 管理员 |
| `recorder` | `123456` | 记录员 |

## 功能模块

1. **登录认证** — 管理员 / 记录员角色，JWT 鉴权
2. **发掘工地 Site** — 名称、时代、经纬度、负责人
3. **探方/发掘单位 Unit** — 所属工地、编号、深度区间、地层简述
4. **出土文物 Find** — 所属探方、登记号、器物类型、材质、完整度、出土日期、描述、存放位置
5. **材质分类 Material** — 名称、描述（字典表）
6. **残片拼合 JoinGroup / JoinMember** — 在文物主数据（finds 表）之上新增的拼合子系统：独立拼合组与成员关联表表达拼合关系，**不另造平行文物主表，也不在 Find 上挂文本字段冒充拼合**
7. **概览页** — 工地数、探方数、文物总数、按器物类型统计

### 残片拼合子系统规则

- `JoinGroup`：`code` 全库唯一；`status` 仅为 `open` / `closed`；含 `title`、`note`。
- `JoinMember`：关联表，`(groupId, findId)` 全库唯一。
- 同一时刻一件文物只能属于一个 **open** 组（可保留已关闭组的历史归属）。
- 入组时若 `Find.completeness` 为「完整」返回 400，仅残缺/碎片可拼合。
- **closed 组禁止再增删成员**（成员关系冻结），仅可删除整个拼合组（只解除关联，不动文物主数据）。
- 组详情页按探方筛选，可勾选同探方残片批量加入；Finds 列表通过只读字段 `joinGroupCode` 显示所属组。

种子数据含 3 个拼合组：`JOIN-ELT1-01`（open，二里头 T1 三片灰陶）、`JOIN-YXT3-01`（open，殷墟 T3 青铜戈两件）、`JOIN-LZT1-01`（closed，良渚 T1 玉琮两件）。

## API 前缀

所有接口以 `/api` 开头：

- `POST /api/auth/login`
- `GET|POST|PUT|DELETE /api/sites`
- `GET|POST|PUT|DELETE /api/units`
- `GET|POST|PUT|DELETE /api/finds`
- `GET|POST|PUT|DELETE /api/materials`
- `GET /api/overview`

残片拼合（均需 JWT）：

- `GET /api/join-groups?status=open|closed` — 列表按状态过滤
- `GET /api/join-groups?findId=12` — 按文物反查所属拼合组（可与 `status` 组合）
- `POST /api/join-groups` / `GET /api/join-groups/:id` / `PUT /api/join-groups/:id` / `DELETE /api/join-groups/:id`
- `POST /api/join-groups/:id/close` — 关闭拼合组
- `POST /api/join-groups/:id/members` — body `{ "findId": 12 }`；完整器 400、closed 400、已属其他 open 组 400
- `DELETE /api/join-groups/:id/members/:findId` — 移出成员；closed 400

前端经 Nginx 将 `/api` 反代至后端容器 `http://backend:8080`。

## 端口映射

| 服务 | 宿主机 | 容器内 |
|------|--------|--------|
| Frontend | 3200 | 80 |
| Backend | 8200 | 8080 |
| MySQL | 3307 | 3306 |

## 目录结构

```
DigCatalog/
├── docker-compose.yml
├── README.md
├── .gitignore
├── backend/
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   └── internal/
│       ├── config/
│       ├── models/
│       ├── handlers/
│       ├── middleware/
│       └── seed/
└── frontend/
    ├── Dockerfile
    ├── nginx.conf
    ├── package.json
    ├── vite.config.js
    ├── index.html
    └── src/
```

## 本地开发（可选）

### 后端

```bash
cd backend
go mod tidy
# 确保 MySQL 已启动且环境变量正确
go run .
```

### 前端

```bash
cd frontend
npm install --registry=https://registry.npmmirror.com
npm run dev
```
