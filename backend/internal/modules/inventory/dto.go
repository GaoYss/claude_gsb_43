package inventory

import (
	"time"

	"streetlight/pkg/pagination"
)

// PartCreateRequest 新增备件与耗材台账请求。
type PartCreateRequest struct {
	Code         string   `json:"code" binding:"required,max=64"`
	Name         string   `json:"name" binding:"required,max=128"`
	Category     string   `json:"category" binding:"max=64"`
	Spec         string   `json:"spec" binding:"max=128"`
	Unit         string   `json:"unit" binding:"max=16"`
	InitialStock *int     `json:"initial_stock" binding:"omitempty,min=0,max=10000000"`
	SafetyStock  *int     `json:"safety_stock" binding:"omitempty,min=0,max=10000000"`
	UnitPrice    *float64 `json:"unit_price" binding:"omitempty,min=0"`
	Supplier     string   `json:"supplier" binding:"max=128"`
	Status       string   `json:"status" binding:"omitempty,oneof=active inactive"`
	Remark       string   `json:"remark" binding:"max=255"`
}

// PartUpdateRequest 修改备件档案请求, 库存只能通过出入库流水变动。
type PartUpdateRequest struct {
	Name        *string  `json:"name" binding:"omitempty,max=128"`
	Category    *string  `json:"category" binding:"omitempty,max=64"`
	Spec        *string  `json:"spec" binding:"omitempty,max=128"`
	Unit        *string  `json:"unit" binding:"omitempty,max=16"`
	SafetyStock *int     `json:"safety_stock" binding:"omitempty,min=0,max=10000000"`
	UnitPrice   *float64 `json:"unit_price" binding:"omitempty,min=0"`
	Supplier    *string  `json:"supplier" binding:"omitempty,max=128"`
	Status      *string  `json:"status" binding:"omitempty,oneof=active inactive"`
	Remark      *string  `json:"remark" binding:"omitempty,max=255"`
}

// InboundRequest 采购(或盘盈)入库登记。
type InboundRequest struct {
	Quantity   int      `json:"quantity" binding:"required,min=1,max=10000000"`
	UnitPrice  *float64 `json:"unit_price" binding:"omitempty,min=0"`
	Supplier   string   `json:"supplier" binding:"max=128"`
	Reason     string   `json:"reason" binding:"max=255"`
	Operator   string   `json:"operator" binding:"max=64"`
	OccurredAt string   `json:"occurred_at" binding:"omitempty,max=32"`
}

// PartScrapRequest 直接从库存报废备件, 必须登记原因。
type PartScrapRequest struct {
	Quantity   int    `json:"quantity" binding:"required,min=1,max=10000000"`
	Reason     string `json:"reason" binding:"required,max=255"`
	Operator   string `json:"operator" binding:"max=64"`
	OccurredAt string `json:"occurred_at" binding:"omitempty,max=32"`
}

// MaterialReturnRequest 维修退料登记, 退回未使用的备件并写明原因。
type MaterialReturnRequest struct {
	Quantity int    `json:"quantity" binding:"required,min=1,max=10000000"`
	Reason   string `json:"reason" binding:"required,max=255"`
	Operator string `json:"operator" binding:"max=64"`
}

// MaterialScrapRequest 维修现场报废登记, 已领用备件损坏/灭失时填写原因。
type MaterialScrapRequest struct {
	Quantity int    `json:"quantity" binding:"required,min=1,max=10000000"`
	Reason   string `json:"reason" binding:"required,max=255"`
	Operator string `json:"operator" binding:"max=64"`
}

// PartListQuery 备件台账查询条件。
type PartListQuery struct {
	pagination.Params
	Keyword  string `form:"keyword"`  // 编号 / 名称 / 规格 / 供应商
	Category string `form:"category"` // 分类精确匹配
	Status   string `form:"status"`
	Shortage bool   `form:"shortage"` // 仅看低于安全库存的备件
}

// TxnListQuery 出入库流水查询条件。
type TxnListQuery struct {
	pagination.Params
	Keyword   string `form:"keyword"` // 备件编号 / 名称 / 流水单号
	TxType    string `form:"tx_type"`
	PartID    uint   `form:"part_id"`
	RepairNo  string `form:"repair_no"`
	FaultNo   string `form:"fault_no"`
	Operator  string `form:"operator"`
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
}

// PartOption 备件下拉选项, 供维修开工选择领用备件。
type PartOption struct {
	ID          uint    `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Spec        string  `json:"spec"`
	Unit        string  `json:"unit"`
	Stock       int     `json:"stock"`
	SafetyStock int     `json:"safety_stock"`
	UnitPrice   float64 `json:"unit_price"`
	Status      string  `json:"status"`
}

// PartOptions 备件下拉数据及建议编号。
type PartOptions struct {
	Items      []PartOption `json:"items"`
	Categories []string     `json:"categories"`
	NextCode   string       `json:"next_code"`
	Units      []string     `json:"units"`
}

// PartStatistics 备件库存统计。
type PartStatistics struct {
	Total           int64   `json:"total"`
	ActiveTotal     int64   `json:"active_total"`
	InactiveTotal   int64   `json:"inactive_total"`
	ShortageTotal   int64   `json:"shortage_total"`
	TotalStockQty   int64   `json:"total_stock_qty"`
	TotalStockValue float64 `json:"total_stock_value"`
}

// ConsumptionRow 备件消耗排名行, 数据来自维修用料明细。
type ConsumptionRow struct {
	PartID        uint    `json:"part_id"`
	PartCode      string  `json:"part_code"`
	PartName      string  `json:"part_name"`
	Spec          string  `json:"spec"`
	Unit          string  `json:"unit"`
	IssuedQty     int     `json:"issued_qty"`     // 累计领用
	ReturnedQty   int     `json:"returned_qty"`   // 累计退料
	ConsumedQty   int     `json:"consumed_qty"`   // 净消耗 = 领用 - 退料(含现场报废)
	RepairCount   int64   `json:"repair_count"`   // 涉及维修单次数
	ConsumedValue float64 `json:"consumed_value"` // 净消耗金额
}

// ShortageItem 缺货预警行。
type ShortageItem struct {
	ID          uint   `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Spec        string `json:"spec"`
	Unit        string `json:"unit"`
	Stock       int    `json:"stock"`
	SafetyStock int    `json:"safety_stock"`
	Gap         int    `json:"gap"` // 距离安全库存的缺口数量
}

// MaterialLine 是维修开工时领用一种备件的明细行。
type MaterialLine struct {
	PartID   uint `json:"part_id" binding:"required"`
	Quantity int  `json:"quantity" binding:"required,min=1,max=10000000"`
}

// RepairRef 是维修模块传给库存模块的维修单快照, 用于流水关联。
type RepairRef struct {
	ID         uint
	RepairNo   string
	FaultID    uint
	FaultNo    string
	LampID     uint
	LampCode   string
	Operator   string
	OccurredAt time.Time
}

// ShortageDetail 描述一条库存不足的备件, 随 409 错误返回给开工接口调用方。
type ShortageDetail struct {
	PartID    uint   `json:"part_id"`
	PartCode  string `json:"part_code"`
	PartName  string `json:"part_name"`
	Unit      string `json:"unit"`
	Need      int    `json:"need"`
	Available int    `json:"available"`
}

// Meta 库存模块字典。
type Meta struct {
	TxTypes      []string `json:"tx_types"`
	PartStatuses []string `json:"part_statuses"`
}
