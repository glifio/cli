package events

type evtCommon struct {
	Error string `json:"error,omitempty"`
	Tx    string `json:"tx,omitempty"`
}

type AgentBorrow struct {
	evtCommon
	AgentID string `json:"agent_id"`
	PoolID  string `json:"pool_id"`
	Amount  string `json:"amount"`
}

type AgentAddMiner struct {
	evtCommon
	AgentID string `json:"agent_id"`
	MinerID string `json:"miner_id"`
}

type AgentMinerChangeOwner struct {
	evtCommon
	AgentID  string `json:"agent_id"`
	MinerID  string `json:"miner_id"`
	OldOwner string `json:"old_owner"`
	NewOwner string `json:"new_owner"`
}

type AgentMinerChangeWorker struct {
	evtCommon
	AgentID    string   `json:"agent_id"`
	MinerID    string   `json:"miner_id"`
	NewWorker  string   `json:"new_worker"`
	NewControl []string `json:"new_control"`
}

type AgentMinerConfirmWorker struct {
	evtCommon
	AgentID string `json:"agent_id"`
	MinerID string `json:"miner_id"`
}

type AgentMinerPull struct {
	evtCommon
	AgentID string `json:"agent_id"`
	MinerID string `json:"miner_id"`
	Amount  string `json:"amount"`
}

type AgentMinerPush struct {
	evtCommon
	AgentID string `json:"agent_id"`
	MinerID string `json:"miner_id"`
	Amount  string `json:"amount"`
}

type AgentMinerReclaim struct {
	evtCommon
	MinerID  string `json:"miner_id"`
	NewOwner string `json:"new_owner"`
}

type AgentMinerRemove struct {
	evtCommon
	AgentID  string `json:"agent_id"`
	MinerID  string `json:"miner_id"`
	NewOwner string `json:"new_owner"`
}

type AgentPay struct {
	evtCommon
	AgentID string `json:"agent_id"`
	PoolID  string `json:"pool_id"`
	Amount  string `json:"amount"`
	PayType string `json:"pay_type"`
}

type AgentWithdraw struct {
	evtCommon
	AgentID string `json:"agent_id"`
	Amount  string `json:"amount"`
	To      string `json:"to"`
}

type AgentExit struct {
	evtCommon
	AgentID string `json:"agent_id"`
	PoolID  string `json:"pool_id"`
	Amount  string `json:"amount"`
}

type WalletFILForward struct {
	evtCommon
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Amount string `json:"amount,omitempty"`
}

type AgentAdmin struct {
	evtCommon
	Action          string `json:"action"`
	AgentID         string `json:"agent_id"`
	NewAdminAddress string `json:"new_admin_address,omitempty"`
}
