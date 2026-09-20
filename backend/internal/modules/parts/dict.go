package parts

var txTypeLabels = map[string]string{
	TxIssue:     "领用",
	TxReturn:    "退料",
	TxScrap:     "报废",
	TxInbound:   "入库",
	TxWriteBack: "冲销",
}

// TxTypeLabel 返回库存变动类型的中文名称。
func TxTypeLabel(value string) string {
	if label, ok := txTypeLabels[value]; ok {
		return label
	}
	return value
}
