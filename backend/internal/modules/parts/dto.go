package parts

import "streetlight/pkg/pagination"

// PartCreateRequest 新增备件台账请求。
type PartCreateRequest struct {
	Code          string   `json:"code" binding:"required,max=64"`
	Name          string   `json:"name" binding:"required,max=128"`
	Category      string   `json:"category" binding:"max=64"`
	Specification string   `json:"specification" binding:"max=128"`
	Unit          string   `json:"unit" binding:"max=16"`
	Stock         *int     `json:"stock" binding:"omitempty,min=0"`
	SafetyStock   *int     `json:"safety_stock" binding:"omitempty,min=0"`
	UnitPrice     *float64 `json:"unit_price" binding:"omitempty,min=0"`
	Supplier      string   `json:"supplier" binding:"max=128"`
	Location      string   `json:"location" binding:"max=64"`
	Remark        string   `json:"remark" binding:"max=255"`
}

// PartUpdateRequest 修改备件台账请求, 指针字段用于区分"未提交"与"置空"。
// 库存不允许通过该接口直接改写, 只能走入库/退料/报废等流水。
type PartUpdateRequest struct {
	Code          *string  `json:"code" binding:"omitempty,max=64"`
	Name          *string  `json:"name" binding:"omitempty,max=128"`
	Category      *string  `json:"category" binding:"omitempty,max=64"`
	Specification *string  `json:"specification" binding:"omitempty,max=128"`
	Unit          *string  `json:"unit" binding:"omitempty,max=16"`
	SafetyStock   *int     `json:"safety_stock" binding:"omitempty,min=0"`
	UnitPrice     *float64 `json:"unit_price" binding:"omitempty,min=0"`
	Supplier      *string  `json:"supplier" binding:"omitempty,max=128"`
	Location      *string  `json:"location" binding:"omitempty,max=64"`
	Remark        *string  `json:"remark" binding:"omitempty,max=255"`
}

// PartListQuery 备件台账查询条件。
type PartListQuery struct {
	pagination.Params
	Keyword  string `form:"keyword"` // 编号 / 名称 / 规格 / 供应商
	Category string `form:"category"`
	Shortage string `form:"shortage"` // all=全部(默认), shortage=仅缺货/预警, out=仅零库存
}

// PartStatistics 备件库存统计。
type PartStatistics struct {
	TotalKinds      int64            `json:"total_kinds"`
	TotalStock      int64            `json:"total_stock"`
	ShortageKinds   int64            `json:"shortage_kinds"`
	OutOfStockKinds int64            `json:"out_of_stock_kinds"`
	TotalStockValue float64          `json:"total_stock_value"`
	ByCategory      map[string]int64 `json:"by_category"`
}

// PartOptions 备件模块下拉选项。
type PartOptions struct {
	Categories []string `json:"categories"`
	Units      []string `json:"units"`
	TxTypes    []string `json:"tx_types"`
}

// StockTxCreateRequest 手工登记库存变动请求(入库/退料/报废)。
// 入库与报废走数量, 退料必须关联维修单(把该单领用的备件退回)。
type StockTxCreateRequest struct {
	PartID     uint   `json:"part_id" binding:"required"`
	Type       string `json:"type" binding:"required,oneof=inbound return scrap"`
	Quantity   int    `json:"quantity" binding:"required,min=1"`
	Reason     string `json:"reason" binding:"max=255"`
	Operator   string `json:"operator" binding:"max=64"`
	OccurredAt string `json:"occurred_at" binding:"omitempty,max=32"`
	RepairID   uint   `json:"repair_id"` // 退料时关联的维修记录
}

// StockTxListQuery 库存流水查询条件。
type StockTxListQuery struct {
	pagination.Params
	Keyword   string `form:"keyword"` // 流水号 / 备件编号 / 备件名称 / 维修单号 / 故障单号
	PartID    uint   `form:"part_id"`
	Type      string `form:"type"`
	Operator  string `form:"operator"`
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
}

// RepairMaterialItem 登记维修(开工)时提交的一种用料。
type RepairMaterialItem struct {
	PartID   uint `json:"part_id" binding:"required"`
	Quantity int  `json:"quantity" binding:"required,min=1"`
}

// ConsumptionRank 备件消耗排名项。
type ConsumptionRank struct {
	PartID      uint    `json:"part_id"`
	PartCode    string  `json:"part_code"`
	PartName    string  `json:"part_name"`
	Unit        string  `json:"unit"`
	TotalQty    int64   `json:"total_qty"`
	RepairCount int64   `json:"repair_count"`
	TotalAmount float64 `json:"total_amount"`
}

// ShortageItem 缺货提示项。
type ShortageItem struct {
	PartID      uint   `json:"part_id"`
	PartCode    string `json:"part_code"`
	PartName    string `json:"part_name"`
	Category    string `json:"category"`
	Unit        string `json:"unit"`
	Stock       int    `json:"stock"`
	SafetyStock int    `json:"safety_stock"`
}
