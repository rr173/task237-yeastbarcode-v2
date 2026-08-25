# 酵母条形码谱系污染判别服务

合成生物实验人员依据条形码测序读段判断培养谱系是否发生交叉污染。服务导入培养代次、条形码读段与稀释记录，执行纠错归并、构建代次继承关系、识别外来条形码证据，并支持锁定祖先克隆、裁决污染候选与发布不可变判别快照。

## 业务闭环

1. 创建培养谱系（建档）。
2. 导入各代次的条形码读段（含质量分数），非法字符或缺失质量被拒绝。
3. 对每代次执行纠错归并：低质量读段排除，其余按编辑距离归并到规范条形码簇。
4. 声明代次继承边（父→子），拒绝循环；锁定奠基代次为祖先克隆。
5. 分析某代次：比较规范条形码与继承期望（祖先+父代），将超出低频阈值且不可由突变解释的外来条形码标记为污染候选。
6. 研究者裁决候选（确认/否决），发布判别快照并封存谱系。

## 标准命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...
go run ./cmd/yeastbc --smoke-test      # 端到端自检
go run ./cmd/yeastbc --addr :8080 --db yeastbc.db
```

## 目录结构

```
cmd/yeastbc/main.go        入口（--addr / --db / --smoke-test）
internal/model             实体、状态机、校验、指纹
internal/store             SQLite 持久化（事务 + 迁移 + 幂等）
internal/read              读段校验与摄入、质量策略
internal/correction        条形码编辑距离、纠错归并
internal/lineage           代次继承图、环检测、祖先锁定
internal/discriminate      外来条形码污染判别与裁决
internal/snapshot          判别快照封存
internal/service           编排层（状态机 + 流程）
internal/httpapi           HTTP 层（/api 前缀）
```

## 持久化与重启恢复

SQLite 表：`lineages`、`reads`、`generation_edges`、`correction_clusters`、`candidates`、`snapshots`。读段内容哈希为幂等键；封存快照冻结祖先假设与全部输入。重启后从 SQLite 恢复，未完成归并可重跑。
