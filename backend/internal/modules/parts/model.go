package parts

import "time"

// 库存变动类型。
const (
	TxIssue     = "issue"     // 领用(维修开工自动扣减)
	TxReturn    = "return"    // 退料(领用后退回, 回补库存)
	TxScrap     = "scrap"     // 报废(登记原因, 库存扣减)
	TxInbound   = "inbound"   // 入库(采购/盘盈入库, 回补库存)
	TxWriteBack = "writeback" // 冲销(删除维修记录时自动回补开工领用)
)

// TxDirections 变动类型对应的库存方向: +1 回补, -1 扣减。
var TxDirections = map[string]int{
	TxIssue:     -1,
	TxReturn:    1,
	TxScrap:     -1,
	TxInbound:   1,
	TxWriteBack: 1,
}

// TxTypes 返回全部库存变动类型。
func TxTypes() []string {
	return []string{TxIssue, TxReturn, TxScrap, TxInbound, TxWriteBack}
}

// IsValidTxType 校验库存变动类型。
func IsValidTxType(value string) bool {
	_, ok := TxDirections[value]
	return ok
}

// IsStockIn 是否为回补库存的变动。
func IsStockIn(value string) bool {
	return TxDirections[value] > 0
}

// Part 备件台账, 一条记录对应一种可库存的备件/耗材, 当前库存以变动流水为唯一增减依据。
type Part struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Code          string    `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name          string    `gorm:"size:128;index;not null" json:"name"`
	Category      string    `gorm:"size:64;index" json:"category"`
	Specification string    `gorm:"size:128" json:"specification"`
	Unit          string    `gorm:"size:16;not null;default:个" json:"unit"`
	Stock         int       `gorm:"not null;default:0" json:"stock"`
	SafetyStock   int       `gorm:"not null;default:0" json:"safety_stock"`
	UnitPrice     float64   `gorm:"not null;default:0" json:"unit_price"`
	Supplier      string    `gorm:"size:128" json:"supplier"`
	Location      string    `gorm:"size:64" json:"location"`
	Remark        string    `gorm:"size:255" json:"remark"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (Part) TableName() string { return "part" }

// Shortage 库存低于(含)安全库存即视为缺货/预警。
func (p *Part) Shortage() bool { return p.Stock <= p.SafetyStock }

// StockTx 备件库存变动流水, 每一次领用/退料/报废/入库/冲销都会登记一条。
type StockTx struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TxNo       string    `gorm:"size:64;uniqueIndex;not null" json:"tx_no"`
	PartID     uint      `gorm:"index;not null" json:"part_id"`
	PartCode   string    `gorm:"size:64;index" json:"part_code"`
	PartName   string    `gorm:"size:128" json:"part_name"`
	Type       string    `gorm:"size:32;index;not null" json:"type"`
	Quantity   int       `gorm:"not null" json:"quantity"` // 变动数量, 恒为正数, 方向由 type 决定
	StockAfter int       `gorm:"not null" json:"stock_after"`
	RepairID   *uint     `gorm:"index" json:"repair_id,omitempty"`
	RepairNo   string    `gorm:"size:64;index" json:"repair_no,omitempty"`
	FaultNo    string    `gorm:"size:64;index" json:"fault_no,omitempty"`
	Reason     string    `gorm:"size:255" json:"reason"`
	Operator   string    `gorm:"size:64;index" json:"operator"`
	OccurredAt time.Time `gorm:"index;not null" json:"occurred_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName 指定表名。
func (StockTx) TableName() string { return "stock_tx" }

// RepairMaterial 维修用料明细, 一条记录对应一次维修开工领用的一种备件。
type RepairMaterial struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RepairID  uint      `gorm:"uniqueIndex:uniq_repair_part;not null" json:"repair_id"`
	PartID    uint      `gorm:"uniqueIndex:uniq_repair_part;not null" json:"part_id"`
	PartCode  string    `gorm:"size:64" json:"part_code"`
	PartName  string    `gorm:"size:128" json:"part_name"`
	Unit      string    `gorm:"size:16" json:"unit"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	UnitPrice float64   `gorm:"not null;default:0" json:"unit_price"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名。
func (RepairMaterial) TableName() string { return "repair_material" }
