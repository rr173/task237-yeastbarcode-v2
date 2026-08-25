基于 Go 实现的酵母条形码谱系污染判别 Web 项目，一款后端服务，完成条形码读段纠错归并、代次继承图构建、外来条形码低频污染判别与判别快照封存。

# BENZHI 评测说明

本项目为单进程 HTTP 后端服务，使用 SQLite 持久化全部业务状态，完成合成生物学实验中培养谱系的交叉污染判别。

## 启动命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/yeastbc --addr :8080 --db yeastbc.db
```

## 自检契约（Docker 唯一判据）

```bash
go run ./cmd/yeastbc --smoke-test
```

`--smoke-test` 不启动长驻服务，而是：导入读段→纠错归并→构建代次继承→锁定祖先→分析外来条形码→裁决候选→发布并封存快照→关闭数据库→重新打开验证持久化与重启恢复，最后以 0 退出码结束。Docker `CMD` 固定为 `--smoke-test`。

## Docker 双架构

```bash
bash build_benzhi_docker.sh docker-baseline-env linux/amd64
bash build_benzhi_docker.sh docker-baseline-env linux/arm64
docker run --rm docker-baseline-env:amd64 --smoke-test
docker run --rm docker-baseline-env:arm64 --smoke-test
```

## API 入口（前缀 /api）

- `GET  /api/healthz`、`GET /api/self-check`
- `GET|POST /api/lineages`、`GET /api/lineages/{id}`、`POST /api/lineages/{id}/transition`
- `GET|POST /api/lineages/{id}/reads`、`GET /api/lineages/{id}/generations/{gen}/reads`
- `POST /api/lineages/{id}/edges`、`POST /api/lineages/{id}/generations/{gen}/correct`
- `GET /api/lineages/{id}/generations/{gen}/clusters`、`POST /api/lineages/{id}/ancestor`
- `POST /api/lineages/{id}/generations/{gen}/analyze`、`GET /api/lineages/{id}/candidates`
- `POST /api/candidates/{cid}/decide`
- `GET|POST /api/lineages/{id}/snapshots`、`GET|POST /api/snapshots/{sid}`（`/confirm`）
