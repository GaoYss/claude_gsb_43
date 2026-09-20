package inventory

import "time"

// 出入库流水类型。
const (
	TxInbound   = "inbound"    // 采购入库
	TxIssue     = "issue"      // 维修领用(开工自动扣减)
	TxReturn    = "return"     // 退料(未使用备件退回可用库存)
	TxScrap     = "scrap"      // 报废(库存报废或维修现场报废, 必须登记原因)
	TxRollback  = "rollback"   // 删除维修记录自动回退
	TxAdjustOut = "adjust_out" // 盘亏调整
)

// 备件状态。
const (
	PartStatusActive   = "active"   // 在用
	PartStatusInactive = "inactive" // 停用
)

// TxTypes 返回全部出入库类型取值。
func TxTypes() []string {
	return []string{TxInbound, TxIssue, TxReturn, TxScrap, TxRollback, TxAdjustOut}
}

// IsValidTxType 校验出入库类型取值。
func IsValidTxType(txType string) bool {
	for _, item := range TxTypes() {
		if item == txType {
			return true
		}
	}
	return false
}

// PartStatuses 返回全部备件状态取值。
func PartStatuses() []string { return []string{PartStatusActive, PartStatusInactive} }

// IsValidPartStatus 校验备件状态取值。
func IsValidPartStatus(status string) bool {
	for _, item := range PartStatuses() {
		if item == status {
			return true
		}
	}
	return false
}

// SparePart 备件与耗材台账, 库存数量以最小计量单位(Unit)计数, 只能通过出入库流水变动。
type SparePart struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"size:128;index;not null" json:"name"`
	Category    string    `gorm:"size:64;index" json:"category"`
	Spec        string    `gorm:"size:128" json:"spec"`
	Unit        string    `gorm:"size:16;not null;default:个" json:"unit"`
	Stock       int       `gorm:"not null;default:0" json:"stock"`
	SafetyStock int       `gorm:"not null;default:0" json:"safety_stock"`
	UnitPrice   float64   `gorm:"not null;default:0" json:"unit_price"`
	Supplier    string    `gorm:"size:128" json:"supplier"`
	Status      string    `gorm:"size:32;index;not null;default:active" json:"status"`
	Remark      string    `gorm:"size:255" json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (SparePart) TableName() string { return "spare_part" }

// StockShortage 库存是否已触及缺货预警线(小于等于安全库存)。
func (p *SparePart) StockShortage() bool {
	return p.Status == PartStatusActive && p.Stock <= p.SafetyStock
}

// StockTransaction 出入库流水, 库存的每一次增减都留痕, 并可与维修记录双向关联。
type StockTransaction struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TxNo        string    `gorm:"size:64;uniqueIndex;not null" json:"tx_no"`
	TxType      string    `gorm:"size:32;index;not null" json:"tx_type"`
	PartID      uint      `gorm:"index;not null" json:"part_id"`
	PartCode    string    `gorm:"size:64;index" json:"part_code"`
	PartName    string    `gorm:"size:128" json:"part_name"`
	Spec        string    `gorm:"size:128" json:"spec"`
	Unit        string    `gorm:"size:16" json:"unit"`
	Quantity    int       `gorm:"not null" json:"quantity"`              // 业务数量, 恒为正数
	StockDelta  int       `gorm:"not null;default:0" json:"stock_delta"` // 实际库存增减(可正可负, 现场报废为 0)
	StockBefore int       `gorm:"not null" json:"stock_before"`
	StockAfter  int       `gorm:"not null" json:"stock_after"`
	UnitPrice   float64   `json:"unit_price"`
	Reason      string    `gorm:"size:255" json:"reason"`
	RepairID    *uint     `gorm:"index" json:"repair_id"`
	RepairNo    string    `gorm:"size:64;index" json:"repair_no"`
	FaultNo     string    `gorm:"size:64;index" json:"fault_no"`
	Operator    string    `gorm:"size:64;index" json:"operator"`
	OccurredAt  time.Time `gorm:"index;not null" json:"occurred_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 指定表名。
func (StockTransaction) TableName() string { return "stock_transaction" }

// RepairMaterial 维修用料明细, 一条记录对应一次维修对一种备件的领用, 随维修记录长期保留。
type RepairMaterial struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	RepairID    uint      `gorm:"index;not null" json:"repair_id"`
	RepairNo    string    `gorm:"size:64;index" json:"repair_no"`
	FaultID     uint      `gorm:"index" json:"fault_id"`
	FaultNo     string    `gorm:"size:64;index" json:"fault_no"`
	LampID      uint      `gorm:"index" json:"lamp_id"`
	LampCode    string    `gorm:"size:64;index" json:"lamp_code"`
	PartID      uint      `gorm:"index;not null" json:"part_id"`
	PartCode    string    `gorm:"size:64" json:"part_code"`
	PartName    string    `gorm:"size:128" json:"part_name"`
	Spec        string    `gorm:"size:128" json:"spec"`
	Unit        string    `gorm:"size:16" json:"unit"`
	Quantity    int       `gorm:"not null" json:"quantity"`               // 开工领用量
	ReturnedQty int       `gorm:"not null;default:0" json:"returned_qty"` // 累计退料数量
	ScrappedQty int       `gorm:"not null;default:0" json:"scrapped_qty"` // 累计现场报废数量
	UnitPrice   float64   `json:"unit_price"`
	Subtotal    float64   `json:"subtotal"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (RepairMaterial) TableName() string { return "repair_material" }

// ConsumedQty 实际净消耗 = 领用量 - 退料量(报废的备件不再退回, 计入消耗)。
func (m *RepairMaterial) ConsumedQty() int {
	consumed := m.Quantity - m.ReturnedQty
	if consumed < 0 {
		return 0
	}
	return consumed
}

// OpenQty 仍挂在维修单上(既未退料也未报废)的数量。
func (m *RepairMaterial) OpenQty() int {
	open := m.Quantity - m.ReturnedQty - m.ScrappedQty
	if open < 0 {
		return 0
	}
	return open
}
