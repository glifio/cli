<img src="./logo.png" alt="GLIF Logo" align="right" width="60px" />

# GLIF CLI

![Github Actions][gha-badge] [![Discord Channel][discord-badge]](https://discord.gg/qKF9HN9a2M)

[gha-badge]: https://img.shields.io/github/actions/workflow/status/glifio/cli/test.yml?branch=main
[discord-badge]: https://dcbadge.vercel.app/api/server/5qsJjsP3Re?style=flat-square&theme=clean-inverted&compact=true&theme=blurple

For English README, please click [here](https://github.com/glifio/glif/blob/main/README.md).

**GLIF 命令行界面是用于与 GLIF Pools 协议互动的工具**

- [GLIF CLI](#glif-cli)
  - [安装](#安装)
    - [使用go安装](#使用go安装)
    - [Linux（即将推出）](#Linux即将推出)
    - [MacOS（即将推出）](#macos即将推出)
    - [从源码构建](#从源码构建)
  - [命名的钱包账户和地址](#命名的钱包账户和地址)
  - [钱包](#钱包)
    - [列出现有的钱包账户和余额](#列出现有的钱包账户和余额)
    - [为使用 Agent 创建钱包账户](#为使用-agent-创建钱包账户)
    - [通用钱包账户](#通用钱包账户)
    - [密码短语](#密码短语)
    - [导入/导出/移除账户](#导入导出移除账户)
    - [从旧版的 keystore.toml 钱包迁移](#从旧版的-keystoretoml-钱包迁移)
  - [代理（Agent）-- 开始借款](#代理agent---开始借款)
    - [创建 Agent](#创建-agent)
    - [给 Agent 添加矿工](#给-agent-添加矿工)
    - [借款](#借款)
    - [在 Miner 和 Agent 之间转移 FIL](#在-miner-和-agent-之间转移-fil)
    - [提取奖励 / 预支现金](#提取奖励--预支现金)
    - [从 Agent 中移除矿工](#从-agent-中移除矿工)
    - [付款](#付款)
    - [付款类型](#付款类型)
    - [自动驾驶](#自动驾驶)
    - [退出池子](#退出池子)
  - [Agent 健康状态](#agent-健康状态)
  - [高级模式](#高级模式)
    - [重置 Agent 的所有者（owner）密钥](#重置-agent-的所有者owner密钥)
    - [重置 Agent 的操作员（operator）密钥](#重置-agent-的操作员operator密钥)
    - [重置 Agent 的请求者（requester）密钥](#重置-agent-的请求者requester密钥)

<hr />

## 安装

### 使用go安装

如果你已经安装了go 1.21版本，可以使用go安装程序轻松安装GLIF CLI：<br />
`go install github.com/glifio/glif/v2@latest`

### Linux（即将推出）

### MacOS（即将推出）

### 从源码构建

如用从源码构建，你需要安装go 1.21或更高版本。

首先，从 GitHub 克隆仓库: <br />
`git clone git@github.com:glifio/glif.git`<br />
`cd cli`<br />

**主网安装**
`make glif`<br />
`sudo make install`<br />
`make config`<br />

**测试网安装**
`make calibnet`<br />
`sudo make install`<br />
`make calibnet-config`<br />

## 命名的钱包账户和地址

GLIF CLI 将人类可读的名称映射到账户地址。每当您向命令传递一个 `address` 参数或标志时，您可以使用名称的人类可读版本。例如，如果您有一个名为 `testing-account` 的账户，您可以通过 `from` `testing-account` 发送交易：

`glif <command> <command-args> --from testing-account`<br />

为任意地址创建只读标签：<br />
`glif wallet label-account <name> <address>`<br />

请注意，如果您添加了内置 actor 的地址（`f1/f2/f3`），它将被转换为 `f0` ID 地址并编码为 `0x` EVM 地址格式。当与 FEVM 上的智能合约交互时，使用 `0x` 格式地址。更多相关信息，请在[这里](https://docs.filecoin.io/smart-contracts/filecoin-evm-runtime/address-types/#converting-to-a-0x-style-address)阅读。

列出所有账户，包括只读标签的账户：<br />
`glif wallet list --include-read-only`

## 钱包

GLIF CLI 内嵌了一个用于向 Filecoin 写入交易的钱包。该钱包基于 go-ethereum 的加密密钥库（[go-ethereum's encrypted keystore](https://geth.ethereum.org/docs/developers/dapp-developer/native-accounts)）。单个“钱包”可以持有多个独立的“账户”，每个“账户”都有一个人类可读的名称。

加密的账户信息存储在 `~/.glif/keystore`，相对应的人类可读名称地址存储在 `~/.glif/accounts`中。

请注意，所有钱包账户都是 EVM actor 类型，这意味着它们在 Filecoin 上有一个 0x/f4 地址。GLIF CLI 钱包尚不支持 f1/f2/f3 样式的地址。

### 列出现有的钱包账户和余额

`glif wallet list`<br />

包括：

`glif wallet balance`<br />

### 为使用 Agent 创建钱包账户

`glif wallet create-agent-accounts`

此命令将创建 3 个新的钱包账户：(1) `owner`，(2) `operator` 和 (3) `requester`，它们对应于一个 Agent (代理)智能合约。在我们的[文档](https://docs.glif.io/agents/owner-and-address-keys)中了解更多关于这些密钥的信息。

**强烈建议安全地备份您的 `owner` 加密密钥 - 失去此密钥意味着失去对您 Agent 的访问权**。

### 通用钱包账户

您还可以创建通用命名的钱包，以在其他命令中使用：<br />
`glif wallet create-account <account-name>`

### 密码短语

钱包账户可以通过独特的密码短语进行额外的安全保护。私钥使用密码短语加密，因此即使攻击者获得了您的 GLIF CLI Keystore，也难以获得您账户的私钥。**强烈建议您使用安全的密码短语保护您的钱包账户**。

### 导入/导出/移除账户

你可以轻松导入、导出和移除钱包中的账户。导入和/或导出账户时，支持原始私钥格式和密码短语加密的密钥格式。请参见下文以获取更多信息。

- 导出私钥，加密你的密码短语：`glif wallet export-account <account-name> --really-do-it`  
  请注意，你需要密码才能将账户重新导入钱包中。
- 导出未加密的原始私钥（危险）：`glif wallet export-account-raw <account-name> --really-do-it`  
- 导入密码短语加密的私钥：`glif wallet import-account <account-name> <hex-encrypted-keyfile>`  
- 导入原始的、十六进制编码的私钥：`glif wallet import-account-raw <account-name> <hex-raw-key>`  
- 从密钥存储中移除账户：`glif wallet remove-account <account-name> --reall-do-it`

**请注意，如果您忘记了密码短语，您的私钥将无法恢复。把您的密码短语写在一个安全的地方，避免被盗或丢失是非常重要的。**

您可以随时通过以下方式更改您的密码短语：<br />
`glif wallet change-passphrase <account-name>`<br />

### 从旧版的 keystore.toml 钱包迁移

如果您使用的是旧版本命令行，您的原始、未加密的私钥是在 `~/.glif/keys.toml` 文件中存储。您也还没有加密的密钥库。可以通过以下方式迁移到新的加密密钥库：<br />

`glif wallet migrate`

在您迁移了钱包之后，我们建议测试一两个命令以确保迁移顺利。迁移成功后，可以安全地删除您的 `keys.toml` 文件：<br />

`shred -fuzv ~/.glif/keys.toml`

## 代理（Agent）-- 开始借款

Agent 是 [GLIF Pools Protocol](https://docs.glif.io/v/zhong-wen/) （构建 Infinity Pool 的协议）的关键组件 - Agent 是一个或多个 [Miner Actors](https://github.com/filecoin-project/specs-actors/blob/master/actors/builtin/miner/miner_actor.go) 的包装合约。Agent 是存储供应商与池子交互的工具。不久之后，我们的网站上将提供 Agent 命令。

### 创建 Agent

创建 Agent 的第一步是创建 Agent 钱包账户：<br />

`glif wallet create-agent-accounts`

接下来，您需要为您的 Agent 的所有者密钥提供资金以支付 gas 费。您可以通过以下命令获取您的 Agent 所有者账户：<br />
`glif wallet list`

给您的账户提供资金，您可以转到 [GLIF Wallet](https://glif.io/wallet)，并向您的所有者地址转送一些资金。**重要提示** - 不要手动制作和发送`method 0`发送交易给 EVM 地址，并传递其`value`。请改用 [fil-forwarder](https://docs.filecoin.io/smart-contracts/filecoin-evm-runtime/filforwader/)。

一旦你为你的所有者密钥提供了资金，请验证：

```
➜ glif wallet balance

Agent accounts:

owner balance: 1.00 FIL
operator balance: 0.00 FIL
requester balance 0.00 FIL
```

最后一步是创建您的 Agent：<br />
`glif agent create`<br />

如果一切顺利，您可以运行：<br />
`glif agent info`<br />

这将打印出您的 Agent 信息。

### 给 Agent 添加矿工

向 Agent 添加矿工，您需要使 Agent 成为您矿工的所有者。此过程分为两个步骤：

1. 向 Miner Actor 提交所有权变更，并将 Agent 的`f4` Filecoin 地址传递为新的所有者。
2. 从 Agent 那里批准所有权变更。

**第一步 - 提交所有权变更**

此步骤在 GLIF 和命令行之外进行。根据您使用的 mining 软件，这一步可能会有所不同。但是，如果您正在运行`lotus-miner`命令行，您可以运行以下命令来提交所有权变更：<br />

`lotus-miner actor set-owner --really-do-it <agent-f410> <current-miner-owner>`<br />

运行`glif agent info`来找到您 Agent 的`f4`地址：

```
➜ glif agent info

BASIC INFO

...
Agent f4 Addr                         f410fh3njwnl6uirpnvi2o7qtnki43c47iyn5mf2q3nq
...
```

一旦此交易成功，您可以进行第二步。

**第二步 - 批准所有权变更**

为了完成向您的 Agent 添加 Miner 的过程，Agent 必须批准所有权变更。批准所有权变更，运行命令：<br />

`glif agent miners add <miner-id>`<br />

一个 Agent 可以拥有一个以上的 Miner，这增加了存储提供商在单个 Agent 下可以借用的总额度。

### 借款

一旦您的 Agent 质押了一个 Miner，您可以运行`glif agent preview borrow-max`来获取您的最大借款额度。请注意，此信息也可以通过运行命令`glif agent info`获得。

当您决定借用多少时，只需运行：<br />
`glif agent borrow <amount>`<br />

交易确认后，FIL 将在您的 Agent 智能合约上可用。请查看下一节，了解如何将资金推送到您 Agent 上的某个 Miner。

注意 - 为了借款，您的 Agent 必须在过去 24 小时内至少为其欠的费用向池中支付回款。

### 在 Miner 和 Agent 之间转移 FIL

您可以直接从 Agent 推送资金到由该 Agent 拥有的 Miner 上，用作在 Filecoin 网络上的抵押品：<br />
`glif agent miners push-funds <miner-id> <amount>`<br />

您可以更改`~/.lotusminer/config.toml`，来使用可用的矿工余额作为扇区抵押，而不是每条消息都发送它：<br />

```
  # Whether to use available miner balance for sector collateral instead of sending it with each message
  #
  # type: bool
  # env var: LOTUS_SEALING_COLLATERALFROMMINERBALANCE
  #CollateralFromMinerBalance = false
```

当您从 Miner 提取资金到 Agent，以提取奖励或进行每周付款时，您可以使用命令：<br />
`glif agent miners pull-funds <miner-id> <amount>`<br />

### 提取奖励 / 预支现金

有时您可能需要一些 FIL 来支付 gas 费或在交易所出售来兑换法币。在这种情况下，您希望从您的 Agent 中提取资金，并从 GLIF Pools 协议中提取。当您的 Agent 有过多的权益时，是可以这样操作 - 要了解更多关于经济学的内容，请参阅我们的[文档](https://docs.glif.io/v/zhong-wen/cun-chu-ti-gong-shang-jing-ji-xue/ti-qu-zi-jin)。

从您的 Agent 中提款：<br />
`glif agent withdraw <amount> <receiver>`<br />

请记住，`receiver`可以是一个钱包账户。例如，您可以将资金提取到您 Agent 的所有者密钥中：<br />

`glif agent withdraw <amount> owner`

### 从 Agent 中移除矿工

您可以通过调用 `glif agent miners remove <miner-id> <new-owner-address>` 将 Miner 从您的 Agent 中移除。此调用将提议更改 Agent Miner 的所有权，并传递 `new-owner-address` 作为新的所有者。此交易成功后，将需要从 `new-owner-address` 批准所有权更改。需要注意的是，如果尝试在 Miner 上设置 EVM actor 作为新所有者，此调用将失败。

需要注意的是，从 Agent 中移除 Miner 就是移除权益，所以如果您由于抵押要求不被允许移除矿工，此调用可能会失败。规则与从您的 Agent 中提取资金相同 - 您可以在[这里](https://docs.glif.io/v/zhong-wen/cun-chu-ti-gong-shang-jing-ji-xue/ti-qu-zi-jin)了解更多关于经济学的内容。

### 付款

在借款之后，存储提供商预期每周支付一次，支付在限定时间段内积累的费用金额。您不受限于每周支付一次 - 您可以每天、每隔一天或每周支付一次。您支付的费用金额与您的付款频率没有直接联系。

进行付款，您的 Agent 必须有足够的余额（资金从 Agent 移回到池中）：<br />
`glif agent pay <payment-type>`<br />

### 付款类型

目前有 3 种付款方式：

1. `to-current` - 仅支付当前欠款的费用
2. `principal` - 支付当前欠款的费用和特定金额的本金
3. `custom` - 支付自定义金额。如果金额大于当前欠款的费用，其余付款应用于本金。

请注意，如果您过多支付了本金，超额支付部分的金额将退还给您的 Agent。所以不能超出支付您所欠的金额。

### 自动驾驶

每周都需要手动付款是非常麻烦，这就是为什么我们开发了自动驾驶功能。自动驾驶是一个服务，可以自动执行：(1) 从您 Agent 的 Miner 处提取资金，并 (2) 还款到池中。

您可以在 `~/.glif/config.toml` 中找到自动驾驶的配置设置。默认设置如下：

```
[autopilot]
# <to-current|principal|custom>
payment-type = 'to-current'
# amount is only required for 'principal' and 'custom' payment types
amount = 0
frequency = 5

[autopilot.pullfunds]
enabled = true
# to save on gas fees, pull the payment amount * pull-amount-factor
pull-amount-factor = 3
# miner that will have funds pulled from it
miner = '<miner-id>'
```

您可以根据自己的需求配置自动驾驶的设置，开始设置，执行命令：<br />
`glif agent autopilot`

### 退出池子

如果您想永久离开池子，您只需要还清所有的本金。我们强烈推荐使用以下命令：<br />

`glif agent exit`<br />

这样可以确保还清 _所有_ 的本金，并且不会留下任何微小数量的 attofil。

## Agent 健康状态

需要注意的是，如果 Agent 开始积累有缺陷的扇区和/或错过其每周付款，它可能进入“不健康”的状态。

如果您的 Agent 被标记为故障状态，`glif agent info` 会告诉您。如果您已从故障状态中恢复，使用以下命令恢复您 Agent 的健康状态：<br />

`glif agent set-recovered`

## 高级模式

GLIF CLI 可以在"高级模式"下构建，这允许您对您的 Agent 进行所有权和管理权更改。要在高级模式下构建 CLI，请运行：<br />
`make advanced`<br />
`sudo make install`<br />

在高级模式下运行时，是能够看到 `glif agent admin` 命令。

### 重置 Agent 的所有者（owner）密钥

1. 首先，生成一个新的账户，该账户将充当 Agent 的新所有者，运行： <br />`glif wallet create-account new-owner`。<br /> 这将在您的 `~/.glif/accounts.toml` 中创建一个新的键值对。当您运行 `glif wallet list` 时，就能看到该账户。
2. **安全地备份您的 new-owner 密钥库文件和（可选的）密码短语。**<br />失去这个密钥和密码短语就等于失去您的 Miner Actor 的所有者密钥。
3. 接下来，给您的 `new-owner` 密钥发送资金，这样它就可以在 Filecoin 区块链上发送交易。
4. 通过运行以下命令向您的代理提议所有权更改：<br />`glif agent admin transfer-ownership new-owner`
5. 一旦初始的 transfer-ownership（所有权转让） 提议命令得到确认，您需要重新配置您的 `~/.glif/accounts.toml`，以将旧的所有者账户与新的所有者账户互换。您只需重命名密钥即可。您可以在 IDE 中进行此操作。例如：<br />

```
# ~/.glif/accounts.toml BEFORE reconfiguration

owner = '0xEBF92B930245060ce67235F23482De5ef200Df3f'
operator = '0x...'
request = '0x...'
new-owner = '0x5b49f3548592282A1f84c1b2C2c9FA40AF263aCA'
```

```
# ~/.glif/accounts.toml AFTER reconfiguration
# Notice how `owner` became `old-owner` and `new-owner` became `owner`

old-owner = '0xEBF92B930245060ce67235F23482De5ef200Df3f'
operator = '0x...'
request = '0x...'
owner = '0x5b49f3548592282A1f84c1b2C2c9FA40AF263aCA'
```

6. 最后，为了完成所有权转移，运行：<br />`glif agent admin accept-ownership`

如果一切成功，当您运行 `glif agent info` 时，就能看到新的所有者地址。

### 重置 Agent 的操作员（operator）密钥

1. 通过运行以下命令重新创建您的 `operator` 密钥：<br /> `glif agent admin new-key operator`<br />复制您的新操作员密钥以在第 2 步中使用。
2. **安全地备份您的 `operator` keystore 文件和（可选的）密码短语。**
3. 给您的新 `operator` 地址发送一些资金，这样它就可以支付 gas 费。
4. 提议更改 `operator`，运行以下命令：<br />`glif agent admin transfer-operator operator`
5. 批准 `operator` 更改，运行以下命令：<br />`glif agent admin accept-operator`

如果一切顺利，当您运行 `glif agent info` 时，就能看到新的 operator（操作员）地址了。

### 重置 Agent 的请求者（requester）密钥

在重置 Agent 的 requester 密钥时，出于安全考虑，我们不会删除任何旧密钥。相反，我们将重命名您当前的 requester 密钥并用新的替换。这是两个步骤：

1. 通过运行以下命令重新创建您的 `request` 密钥：<br /> `glif agent admin new-key request`<br />复制新 requester 密钥以在第 2 步中使用。
2. 在您的 Agent 上更改 `request` 密钥（这会触发一个链上交易）：<br />`glif agent admin change-requester request`

一旦第二个交易在链上得到确认，您就完成操作了！

如果一切顺利，当您运行 `glif agent info` 时，您应该能看到新的 requester 地址。
