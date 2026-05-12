# TokenBridge 启动与分发说明

## 当前状态

项目当前已经完成以下关键能力：

- 后端可编译、可运行
- SQLite 已切换为纯 Go 驱动，无需 CGO
- Admin 前端已嵌入 Go 二进制
- `/admin/*` 已支持 SPA fallback
- Portable 单目录形态已完成验证
- 可通过 `build/package.ps1` 一键打包

---

## 已验证接口

| 接口 | 方法 | 状态 |
|------|------|------|
| `/health` | GET | ✅ 200 |
| `/v1/models` | GET | ✅ 200 |
| `/admin/api/overview` | GET | ✅ 200 |
| `/admin/` | GET | ✅ 200 |
| `/admin/providers` | GET | ✅ 200 |

---

## 启动方式

### 开发模式

```powershell
cd D:\idea\tokenbridge
D:\Go\bin\go.exe run .\cmd\tokenbridge
```

### 编译后启动

```powershell
cd D:\idea\tokenbridge
D:\Go\bin\go.exe build -o tokenbridge.exe .\cmd\tokenbridge
.\tokenbridge.exe
```

### Portable 模式

```powershell
cd D:\idea\tokenbridge\build\portable\TokenBridge
.\tokenbridge.exe
```

访问地址：

```text
http://127.0.0.1:18743/admin
```

---

## 打包方式

```powershell
cd D:\idea\tokenbridge
powershell -File build\package.ps1
```

打包输出目录：

```text
build/portable/TokenBridge/
```

---

## Portable 目录结构

```text
TokenBridge/
├── tokenbridge.exe
├── config.yaml
├── data/
├── logs/
└── README.txt
```

---

## 配置加载优先级

1. `TB_CONFIG` 环境变量
2. 当前目录 `config.yaml`
3. `configs/config.example.yaml`

---

## 维护检查

改动启动、打包或嵌入资源后，至少执行：

```powershell
go test ./...
cd web\admin
npm run build
```

`npm run build` 会同步前端产物到 `build/embed/admin`，该目录属于生成物，不纳入版本管控。
