package inventory

var txTypeLabels = map[string]string{
	TxInbound:   "采购入库",
	TxIssue:     "维修领用",
	TxReturn:    "维修退料",
	TxScrap:     "备件报废",
	TxRollback:  "删除回退",
	TxAdjustOut: "盘亏调整",
}

var partStatusLabels = map[string]string{
	PartStatusActive:   "在用",
	PartStatusInactive: "停用",
}

// TxTypeLabel 返回出入库类型的中文名称。
func TxTypeLabel(txType string) string {
	if label, ok := txTypeLabels[txType]; ok {
		return label
	}
	return txType
}

// PartStatusLabel 返回备件状态的中文名称。
func PartStatusLabel(status string) string {
	if label, ok := partStatusLabels[status]; ok {
		return label
	}
	return status
}
