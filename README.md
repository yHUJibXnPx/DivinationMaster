# 占卜周易脚本 
开发的占卜算卦脚本，用来测吉凶  

![Watchers](https://img.shields.io/github/watchers/yHUJibXnPx/DivinationMaster) ![Stars](https://img.shields.io/github/stars/yHUJibXnPx/DivinationMaster) ![Forks](https://img.shields.io/github/forks/yHUJibXnPx/DivinationMaster) ![Vistors](https://visitor-badge.laobi.icu/badge?page_id=yHUJibXnPx.DivinationMaster) ![LICENSE](https://img.shields.io/badge/license-MIT-green.svg)

<!-- START_STAR_HISTORY_SELF -->
![Star History Chart](./star_history_self.png)
<!-- END_STAR_HISTORY_SELF -->

## 描述
最近过得不好，找不到工作，事事不顺心，我好痛苦，所以开发了占卜算卦脚本，用来测吉凶，唉😔

周易占卜 + 六比特态向量引擎。两套实现，行为一致：

| | 位置 | 适合 |
|---|---|---|
| **Python 版**（参考实现） | `divination/` | 改引擎、做实验；纯标准库，无需安装依赖 |
| **Go 版**（单文件分发） | `go/` | 日常使用：单个静态二进制，语料已内嵌，不需要任何运行时 |

同一个 `(问题, 种子, 引擎参数)` 在两个版本里得到**同一个结果**（卦、断卦文字、概率分布；整段终端输出逐字一致），两个版本写的 `history.jsonl` 也能互相读取。

```bash
# Python 版
python -m divination "我该接受这份工作吗？"
python -m divination ask "问题" --seed 42 --json   # 可复现、机器可读
python -m divination show 屯                       # 一整卦：卦辞 + 六爻爻辞 + 象传（卦名/序号/卦码均可）
python -m divination history -n 10

# Go 版（用 go/dist/ 里的预编译文件，或自己编译）
./divination-linux-arm64 "我该接受这份工作吗？"     # Termux（Android arm64）
./divination-darwin-arm64 show 屯                  # Apple Silicon
./go/build.sh                                      # 交叉编译四个平台到 go/dist/（需要 Go ≥ 1.24）

./check.sh                                         # 一键校验：语料重建 + 标准答案未过期 + Python 测试 + Go 测试/对拍
```

## 相比 v1 修了什么

| v1 的问题 | v2 |
|---|---|
| `yao.json` 只有 12 条，任何卦的爻辞都相同 | 384 条爻辞 + 乾「用九」、坤「用六」，逐卦逐爻，并带象传 |
| 只有 63 卦；「剥」被错标成观的卦码 `000011` | 64 卦齐全，卦码已校验（上下卦、爻名、序卦"非覆即变"均有测试） |
| 白话在「摘要｜详解」接缝处漏句号（如「得此卦者不可独立创业坤象征大地」） | 27 处已补「。」，有回归测试 |
| `random.choice` 抛"一枚硬币"，没有变爻 | 三枚铜钱，6/7/8/9 四态爻（概率 1:3:3:1），有变爻、变卦 |
| 关键词匹配出"吉 75%"，且 `大吉` 永远走不到 | 删除伪概率；改用朱熹《易学启蒙》变爻读法选取应读经文 |
| 每次读改写整个 history.json | JSON Lines 追加写入；记录含种子，可完全复现 |

## 量子引擎做了什么（`quantum.py` / `go/internal/quantum`）

六爻 = 六个量子比特，64 卦 = 64 个基态，经典计算机上精确模拟（64 个复数）。

1. **制备**：每爻一个量子比特。稳定爻几乎确定（涨落角 `fuzz`），变爻（老阴/老阳）是"本态 ↔ 变态"各 50% 的叠加；翻转分量的相位由 `SHAKE256(种子|问题)` 决定。
2. **纠缠**：在应位（初–四、二–五、三–上）上施加受控相位门。
3. **干涉**：逐比特小角度 Rx，把相位转成概率差异。
4. **测量**：按玻恩规则抽样，坍缩为一个具体的卦；同时展示测量前的概率分布、分布熵、每爻的纠缠熵。

几点必须说清楚：

* **错卦、综卦是酉变换**（置换算子），可作用于态向量；**互卦不是**（64→16 多对一，不可逆），只能作用于测量结果。
* 问题的措辞**只**通过相位影响干涉图样，没有任何语义。
* 这是数学模拟，**不增加任何预测能力**。价值在于呈现"本卦 → 变卦"之间的可能性结构，而不是给出吉凶判决。
* 随机性来自确定性伪随机数（xoshiro256**，种子取自操作系统熵），不是物理上的量子随机。

引擎参数：`--fuzz`（0 = 稳定爻绝对稳定）、`--entangle`（0 = 不纠缠）、`--interfere`（0 = 不干涉）。三者都为 0 时，可能的卦恰是"变爻任意子集翻转"的 2^k 个、且等概率（有测试）。

## 跨语言一致性是怎么保证的

* **PRNG**：两边都用 SplitMix64 播种 + xoshiro256**（只用整数运算，位级一致）；不用各语言自带的随机数库。
* **标准答案**：`tools/gen_golden.py` 用 Python 生成 `testdata/golden.json`（RNG 序列、36 组引擎态、12 组读法、30 组整次占卜、13 组 CLI 输出、历史行）；Go 的 `internal/parity` 逐项对拍。改引擎后须重新生成并让两边都通过；`check.sh` 会拦下"忘了更新"的情况。
* **浮点**：概率分布两边吻合到 1e-12，不保证逐位相同（不同语言/架构的 sin/cos、是否融合乘加会有末位差异）。因此"同种子同卦"在数学上成立，但在某个浮点边界上理论上可能偶发分歧（量级约 1e-12/次）。

## 目录

```
divination/        Python 版（hexagram / quantum / reading / corpus / core / render / history / cli / rng）
  data/iching.json   64 卦 / 386 条爻辞 / 象传 / 白话（唯一语料来源）
go/                Go 版（module divinationmaster，仅标准库）
  cmd/divination     入口
  internal/…         与 Python 模块一一对应；internal/parity 是对拍测试
  build.sh           交叉编译；dist/ 为产物（已 .gitignore）
tools/
  build_data.py      由 tools/sources/ 重建 iching.json，并同步到 go/internal/corpus/
  gen_golden.py      生成 / 检查（--check）跨语言标准答案
testdata/          golden.json、history_sample.jsonl
tests/             Python 单元测试
```

卦码约定：6 位 0/1 字符串，**自初爻到上爻**（自下而上），1 = 阳（与 v1 的 hexagram.json 一致）。

## 数据来源与限制

* 卦辞、爻辞、象传：《周易》经传原文（古籍），整理自 [john-walks-slow/open-iching](https://github.com/john-walks-slow/open-iching)。该仓库未附许可证，自述"采集自互联网，仅供学习交流，不得用于商业用途"；再分发前请自行留意。
* 卦辞白话：沿用 v1 的 `hexagram.json`（已修接缝标点）；第 20 卦「观」v1 缺失，白话为新写。
* **爻辞没有白话解释**（v1 的 `yao.json` 并非逐爻数据，已弃用）。
* 旧的 `history.json` 不会被读取或改动；`history.jsonl` 含问题原文，已加入 `.gitignore`。

## 尚未验证

* 预编译的 linux-arm64 / darwin-* 二进制只做了交叉编译，**没有在真机上运行过**（开发环境是 linux-amd64）。
* Python 只在 3.12 上测试；3.10+ 理论可用但未验证。

## 效果图
![image](assets/0.png)

# 声明
本项目仅作学习交流使用，用于查找资料，学习知识，不做任何违法行为。所有资源均来自互联网，仅供大家交流学习使用，出现违法问题概不负责。
