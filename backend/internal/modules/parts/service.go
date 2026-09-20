package parts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"streetlight/internal/apperr"
	"streetlight/pkg/pagination"
)

// partSortSpec 备件台账允许的排序字段白名单。
var partSortSpec = pagination.SortSpec{
	Allowed: map[string]string{
		"code":         "code",
		"name":         "name",
		"category":     "category",
		"stock":        "stock",
		"safety_stock": "safety_stock",
		"unit_price":   "unit_price",
		"created_at":   "created_at",
		"updated_at":   "updated_at",
	},
	Default: "id",
}

// txSortSpec 库存流水允许的排序字段白名单。
var txSortSpec = pagination.SortSpec{
	Allowed: map[string]string{
		"tx_no":       "tx_no",
		"type":        "type",
		"quantity":    "quantity",
		"operator":    "operator",
		"occurred_at": "occurred_at",
		"created_at":  "created_at",
	},
	Default: "occurred_at",
}

// Service 承载备件台账与库存变动的业务规则。
type Service struct {
	repo *Repository
}

// NewService 构造备件库存服务。
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ---- 备件台账 ----

// GetPart 查询备件详情。
func (s *Service) GetPart(ctx context.Context, id uint) (*Part, error) {
	return s.repo.GetPartByID(ctx, id)
}

// ListParts 分页查询备件台账。
func (s *Service) ListParts(ctx context.Context, query PartListQuery) ([]Part, int64, pagination.Query, error) {
	page := pagination.Parse(query.Params, partSortSpec)
	filter := Filter{
		Keyword:  strings.TrimSpace(query.Keyword),
		Category: strings.TrimSpace(query.Category),
		Shortage: strings.TrimSpace(query.Shortage),
	}
	if filter.Shortage != "" && filter.Shortage != "all" && filter.Shortage != "shortage" && filter.Shortage != "out" {
		return nil, 0, page, apperr.BadRequest("非法的缺货筛选条件: %s", filter.Shortage)
	}
	items, total, err := s.repo.ListParts(ctx, filter, page)
	if err != nil {
		return nil, 0, page, err
	}
	return items, total, page, nil
}

// CreatePart 新增备件台账, 若给定期初库存则同时登记一条入库流水。
func (s *Service) CreatePart(ctx context.Context, req PartCreateRequest) (*Part, error) {
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, apperr.BadRequest("备件编号不能为空")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.BadRequest("备件名称不能为空")
	}
	exists, err := s.repo.ExistsPartByCode(ctx, code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflict("备件编号已存在: %s", code)
	}

	stock := 0
	if req.Stock != nil {
		stock = *req.Stock
	}
	entity := &Part{
		Code:          code,
		Name:          name,
		Category:      strings.TrimSpace(req.Category),
		Specification: strings.TrimSpace(req.Specification),
		Unit:          defaultUnit(req.Unit),
		Stock:         stock,
		SafetyStock:   intOrZero(req.SafetyStock),
		UnitPrice:     floatOrZero(req.UnitPrice),
		Supplier:      strings.TrimSpace(req.Supplier),
		Location:      strings.TrimSpace(req.Location),
		Remark:        strings.TrimSpace(req.Remark),
	}

	err = s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		if err := s.repo.CreatePartTx(tx, entity); err != nil {
			return err
		}
		if stock > 0 {
			_, err := s.writeTx(tx, entity, TxInbound, stock, "期初库存入库", "系统", time.Now(), nil)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entity, nil
}

// UpdatePart 修改备件档案, 库存只能通过流水变动, 不在此接口改写。
func (s *Service) UpdatePart(ctx context.Context, id uint, req PartUpdateRequest) (*Part, error) {
	entity, err := s.repo.GetPartByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Code != nil {
		code := strings.TrimSpace(*req.Code)
		if code == "" {
			return nil, apperr.BadRequest("备件编号不能为空")
		}
		if code != entity.Code {
			exists, err := s.repo.ExistsPartByCode(ctx, code, id)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, apperr.Conflict("备件编号已存在: %s", code)
			}
			entity.Code = code
		}
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperr.BadRequest("备件名称不能为空")
		}
		entity.Name = name
	}
	if req.Category != nil {
		entity.Category = strings.TrimSpace(*req.Category)
	}
	if req.Specification != nil {
		entity.Specification = strings.TrimSpace(*req.Specification)
	}
	if req.Unit != nil {
		entity.Unit = defaultUnit(*req.Unit)
	}
	if req.SafetyStock != nil {
		if *req.SafetyStock < 0 {
			return nil, apperr.BadRequest("安全库存不能小于 0")
		}
		entity.SafetyStock = *req.SafetyStock
	}
	if req.UnitPrice != nil {
		entity.UnitPrice = floatOrZero(req.UnitPrice)
	}
	if req.Supplier != nil {
		entity.Supplier = strings.TrimSpace(*req.Supplier)
	}
	if req.Location != nil {
		entity.Location = strings.TrimSpace(*req.Location)
	}
	if req.Remark != nil {
		entity.Remark = strings.TrimSpace(*req.Remark)
	}
	if err := s.repo.UpdatePart(ctx, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

// DeletePart 删除备件, 已产生库存流水或维修用料的备件不允许删除, 保证台账可追溯。
func (s *Service) DeletePart(ctx context.Context, id uint) error {
	entity, err := s.repo.GetPartByID(ctx, id)
	if err != nil {
		return err
	}
	txCount, err := s.repo.CountStockTxByPart(ctx, id)
	if err != nil {
		return err
	}
	if txCount > 0 {
		return apperr.Conflict("备件 %s 已存在库存变动记录, 不允许删除", entity.Code)
	}
	materialCount, err := s.repo.CountMaterialByPart(ctx, id)
	if err != nil {
		return err
	}
	if materialCount > 0 {
		return apperr.Conflict("备件 %s 已被维修工单领用, 不允许删除", entity.Code)
	}
	return s.repo.DeletePart(ctx, id)
}

// Statistics 汇总备件库存统计。
func (s *Service) Statistics(ctx context.Context) (*PartStatistics, error) {
	totalKinds, err := s.repo.CountParts(ctx)
	if err != nil {
		return nil, err
	}
	totalStock, totalValue, err := s.repo.SumPartStock(ctx)
	if err != nil {
		return nil, err
	}
	shortage, out, err := s.repo.CountShortage(ctx)
	if err != nil {
		return nil, err
	}
	byCategory, err := s.repo.CountPartsByColumn(ctx, "category")
	if err != nil {
		return nil, err
	}
	return &PartStatistics{
		TotalKinds:      totalKinds,
		TotalStock:      totalStock,
		ShortageKinds:   shortage,
		OutOfStockKinds: out,
		TotalStockValue: totalValue,
		ByCategory:      byCategory,
	}, nil
}

// Options 返回备件模块下拉选项与建议编号。
func (s *Service) Options(ctx context.Context) (*PartOptions, error) {
	categories, err := s.repo.DistinctPartValues(ctx, "category")
	if err != nil {
		return nil, err
	}
	units, err := s.repo.DistinctPartValues(ctx, "unit")
	if err != nil {
		return nil, err
	}
	return &PartOptions{
		Categories: categories,
		Units:      units,
		TxTypes:    []string{TxInbound, TxReturn, TxScrap},
	}, nil
}

// ---- 库存流水 ----

// CreateStockTx 手工登记入库 / 退料 / 报废, 与维修开工自动领用走同一套库存增减逻辑。
func (s *Service) CreateStockTx(ctx context.Context, req StockTxCreateRequest) (*StockTx, error) {
	txType := strings.TrimSpace(req.Type)
	if txType != TxInbound && txType != TxReturn && txType != TxScrap {
		return nil, apperr.BadRequest("非法的库存变动类型: %s", txType)
	}
	if req.Quantity <= 0 {
		return nil, apperr.BadRequest("变动数量必须大于 0")
	}
	reason := strings.TrimSpace(req.Reason)
	switch txType {
	case TxReturn:
		if reason == "" {
			return nil, apperr.BadRequest("退料必须登记退料原因")
		}
	case TxScrap:
		if reason == "" {
			return nil, apperr.BadRequest("报废必须登记报废原因")
		}
	}

	occurredAt, err := parseTime(req.OccurredAt, time.Now())
	if err != nil {
		return nil, err
	}
	operator := strings.TrimSpace(req.Operator)

	var result *StockTx
	err = s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		part, err := s.repo.GetPartByIDTx(tx, req.PartID)
		if err != nil {
			return err
		}

		var ref *RepairRef
		if txType == TxReturn {
			if req.RepairID == 0 {
				return apperr.BadRequest("退料必须关联对应的维修记录")
			}
			ref, err = s.loadRepairRef(tx, req.RepairID)
			if err != nil {
				return err
			}
			if err := s.validateReturnQuantity(tx, part.ID, req.RepairID, req.Quantity); err != nil {
				return err
			}
		}

		if txType == TxScrap && part.Stock < req.Quantity {
			return apperr.Conflict("备件 %s(%s) 库存不足, 无法报废: 需报废 %d%s, 当前可用 %d%s",
				part.Name, part.Code, req.Quantity, part.Unit, part.Stock, part.Unit)
		}

		part.Stock += TxDirections[txType] * req.Quantity
		if err := s.repo.UpdateStockTx(tx, part.ID, part.Stock); err != nil {
			return err
		}
		result, err = s.writeTx(tx, part, txType, req.Quantity, reason, operator, occurredAt, ref)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListStockTx 分页查询库存流水。
func (s *Service) ListStockTx(ctx context.Context, query StockTxListQuery) ([]StockTx, int64, pagination.Query, error) {
	page := pagination.Parse(query.Params, txSortSpec)
	filter := TxFilter{
		Keyword:  strings.TrimSpace(query.Keyword),
		PartID:   query.PartID,
		Type:     strings.TrimSpace(query.Type),
		Operator: strings.TrimSpace(query.Operator),
	}
	if filter.Type != "" && !IsValidTxType(filter.Type) {
		return nil, 0, page, apperr.BadRequest("非法的库存变动类型: %s", filter.Type)
	}
	if value := strings.TrimSpace(query.StartDate); value != "" {
		from, err := parseDay(value)
		if err != nil {
			return nil, 0, page, err
		}
		filter.OccurredFrom = &from
	}
	if value := strings.TrimSpace(query.EndDate); value != "" {
		to, err := parseDay(value)
		if err != nil {
			return nil, 0, page, err
		}
		to = to.AddDate(0, 0, 1)
		filter.OccurredTo = &to
	}
	if filter.OccurredFrom != nil && filter.OccurredTo != nil && filter.OccurredTo.Before(*filter.OccurredFrom) {
		return nil, 0, page, apperr.BadRequest("结束日期不能早于开始日期")
	}
	items, total, err := s.repo.ListStockTx(ctx, filter, page)
	if err != nil {
		return nil, 0, page, err
	}
	return items, total, page, nil
}

// ---- 维修用料 ----

// ListMaterials 查询某次维修的用料明细。
func (s *Service) ListMaterials(ctx context.Context, repairID uint) ([]RepairMaterial, error) {
	return s.repo.ListMaterialsByRepair(ctx, repairID)
}

// ListMaterialsMap 批量查询多次维修的用料明细, 以维修记录 ID 为键返回。
func (s *Service) ListMaterialsMap(ctx context.Context, repairIDs []uint) (map[uint][]RepairMaterial, error) {
	items, err := s.repo.ListMaterialsByRepairs(ctx, repairIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[uint][]RepairMaterial, len(repairIDs))
	for _, item := range items {
		result[item.RepairID] = append(result[item.RepairID], item)
	}
	return result, nil
}

// ConsumptionRanking 返回备件消耗排名。
func (s *Service) ConsumptionRanking(ctx context.Context, limit int) ([]ConsumptionRank, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.ConsumptionRanking(ctx, limit)
}

// ListShortage 返回缺货/预警备件清单。
func (s *Service) ListShortage(ctx context.Context, limit int) ([]Part, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.ListShortage(ctx, limit)
}

// ---- 维修模块端口: 开工领用 / 删除冲销 ----

// CheckIssues 开工前校验用料清单的库存是否充足, 返回每一种不足备件的可用数量。
// items 由维修模块传入, 允许同一备件出现多次(内部合并)。
func (s *Service) CheckIssues(ctx context.Context, items []IssueItem) ([]IssueShortage, error) {
	merged := mergeItems(items)
	shortages := make([]IssueShortage, 0)
	for partID, quantity := range merged {
		part, err := s.repo.GetPartByID(ctx, partID)
		if err != nil {
			return nil, err
		}
		if part.Stock < quantity {
			shortages = append(shortages, IssueShortage{
				PartID:    part.ID,
				PartCode:  part.Code,
				PartName:  part.Name,
				Unit:      part.Unit,
				Requested: quantity,
				Available: part.Stock,
			})
		}
	}
	return shortages, nil
}

// IssueForRepair 在维修开工事务内领用备件: 扣减库存、登记领用流水、写入用料明细。
// 调用方负责保证已通过 CheckIssues; 此处再次强校验, 防止并发超领。
func (s *Service) IssueForRepair(ctx context.Context, tx *gorm.DB, ref RepairRef, items []IssueItem, occurredAt time.Time) error {
	merged := mergeItems(items)
	if len(merged) == 0 {
		return nil
	}

	partIDs := make([]uint, 0, len(merged))
	for id := range merged {
		partIDs = append(partIDs, id)
	}
	parts, err := s.repo.ListPartByIDsTx(tx, partIDs)
	if err != nil {
		return err
	}
	partMap := make(map[uint]*Part, len(parts))
	for index := range parts {
		partMap[parts[index].ID] = &parts[index]
	}

	materials := make([]RepairMaterial, 0, len(merged))
	for partID, quantity := range merged {
		part, ok := partMap[partID]
		if !ok {
			return apperr.NotFound("备件不存在: id=%d", partID)
		}
		if part.Stock < quantity {
			return apperr.Conflict("备件 %s(%s) 库存不足, 无法开工: 需领用 %d%s, 当前可用 %d%s",
				part.Name, part.Code, quantity, part.Unit, part.Stock, part.Unit)
		}
		part.Stock -= quantity
		if err := s.repo.UpdateStockTx(tx, part.ID, part.Stock); err != nil {
			return err
		}
		if _, err := s.writeTx(tx, part, TxIssue, quantity,
			fmt.Sprintf("维修开工领用 %s", ref.RepairNo), ref.Operator, occurredAt, &ref); err != nil {
			return err
		}
		materials = append(materials, RepairMaterial{
			RepairID:  ref.RepairID,
			PartID:    part.ID,
			PartCode:  part.Code,
			PartName:  part.Name,
			Unit:      part.Unit,
			Quantity:  quantity,
			UnitPrice: part.UnitPrice,
		})
	}
	return s.repo.CreateMaterialsTx(tx, materials)
}

// RestoreForRepair 在删除维修记录的事务内冲销开工领用: 回补库存、登记冲销流水、删除用料明细。
func (s *Service) RestoreForRepair(ctx context.Context, tx *gorm.DB, ref RepairRef, reason string) error {
	materials, err := s.repo.ListMaterialsByRepairTx(tx, ref.RepairID)
	if err != nil {
		return err
	}
	for _, material := range materials {
		part, err := s.repo.GetPartByIDTx(tx, material.PartID)
		if err != nil {
			return err
		}
		part.Stock += material.Quantity
		if err := s.repo.UpdateStockTx(tx, part.ID, part.Stock); err != nil {
			return err
		}
		if _, err := s.writeTx(tx, part, TxWriteBack, material.Quantity,
			strings.TrimSpace(reason), "系统", time.Now(), &ref); err != nil {
			return err
		}
	}
	return s.repo.DeleteMaterialsByRepairTx(tx, ref.RepairID)
}

// ---- 内部辅助 ----

// RepairRef 描述库存流水关联的维修单上下文。
type RepairRef struct {
	RepairID uint
	RepairNo string
	FaultNo  string
	Operator string
}

// IssueItem 是维修模块传入的一种待领用备件。
type IssueItem struct {
	PartID   uint `json:"part_id"`
	Quantity int  `json:"quantity"`
}

// IssueShortage 描述一种库存不足的备件。
type IssueShortage struct {
	PartID    uint   `json:"part_id"`
	PartCode  string `json:"part_code"`
	PartName  string `json:"part_name"`
	Unit      string `json:"unit"`
	Requested int    `json:"requested"`
	Available int    `json:"available"`
}

// repairRow 是 repair 表的最小投影, 避免备件模块反向依赖维修模块。
type repairRow struct {
	ID        uint
	RepairNo  string
	FaultNo   string
	Repairman string
	Status    string
}

func (repairRow) TableName() string { return "repair" }

// loadRepairRef 在事务内读取维修单基础信息。
func (s *Service) loadRepairRef(tx *gorm.DB, repairID uint) (*RepairRef, error) {
	var row repairRow
	if err := tx.First(&row, repairID).Error; err != nil {
		return nil, apperr.NotFound("维修记录不存在: id=%d", repairID)
	}
	return &RepairRef{RepairID: row.ID, RepairNo: row.RepairNo, FaultNo: row.FaultNo, Operator: row.Repairman}, nil
}

// validateReturnQuantity 校验退料数量不超过该维修单的实际领用量(扣除已退)。
func (s *Service) validateReturnQuantity(tx *gorm.DB, partID uint, repairID uint, quantity int) error {
	var material RepairMaterial
	err := tx.Where("repair_id = ? AND part_id = ?", repairID, partID).First(&material).Error
	if err != nil {
		if isRecordNotFound(err) {
			return apperr.BadRequest("维修单未领用该备件, 不能退料")
		}
		return fmt.Errorf("查询维修用料失败: %w", err)
	}
	returned, err := s.repo.SumReturnedQuantityTx(tx, repairID, partID)
	if err != nil {
		return err
	}
	if returned+quantity > material.Quantity {
		return apperr.Conflict("退料数量超过可退数量: 本次申请退 %d%s, 已领用 %d%s, 已退 %d%s, 最多可再退 %d%s",
			quantity, material.Unit, material.Quantity, material.Unit, returned, material.Unit,
			material.Quantity-returned, material.Unit)
	}
	return nil
}

// writeTx 生成流水号并落库一条库存流水, 必须在调用方的事务内执行, 保证与库存增减同源。
func (s *Service) writeTx(tx *gorm.DB, part *Part, txType string, quantity int, reason, operator string, occurredAt time.Time, ref *RepairRef) (*StockTx, error) {
	prefix := "KC" + occurredAt.Format("20060102")
	sequence, err := s.repo.NextTxSequenceTx(tx, prefix)
	if err != nil {
		return nil, err
	}

	record := &StockTx{
		TxNo:       fmt.Sprintf("%s%04d", prefix, sequence),
		PartID:     part.ID,
		PartCode:   part.Code,
		PartName:   part.Name,
		Type:       txType,
		Quantity:   quantity,
		StockAfter: part.Stock,
		Reason:     reason,
		Operator:   operator,
		OccurredAt: occurredAt,
	}
	if ref != nil {
		record.RepairID = &ref.RepairID
		record.RepairNo = ref.RepairNo
		record.FaultNo = ref.FaultNo
	}

	// 同事务内并发生成流水号冲突时按序号递增重试。
	for attempt := 0; attempt < 5; attempt++ {
		err = s.repo.CreateStockTx(tx, record)
		if err == nil {
			return record, nil
		}
		if !isUniqueViolation(err) {
			return nil, err
		}
		record.TxNo = fmt.Sprintf("%s%04d", prefix, sequence+attempt+1)
	}
	return nil, apperr.Conflict("库存流水号生成冲突, 请稍后重试")
}

// mergeItems 合并同一备件的重复领用数量。
func mergeItems(items []IssueItem) map[uint]int {
	merged := make(map[uint]int)
	for _, item := range items {
		if item.PartID == 0 || item.Quantity <= 0 {
			continue
		}
		merged[item.PartID] += item.Quantity
	}
	return merged
}

func defaultUnit(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "个"
	}
	return value
}

func intOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func floatOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// parseTime 解析时间字符串, 为空时返回 fallback。
func parseTime(value string, fallback time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, apperr.BadRequest("时间格式不正确, 建议使用 YYYY-MM-DD HH:mm:ss: %s", value)
}

// parseDay 解析 YYYY-MM-DD 日期, 返回当天零点。
func parseDay(value string) (time.Time, error) {
	date, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.Local)
	if err != nil {
		return time.Time{}, apperr.BadRequest("日期格式应为 YYYY-MM-DD, 当前值: %s", value)
	}
	return date, nil
}

// isUniqueViolation 兼容 sqlite 与 postgres 的唯一约束冲突判断。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique violation")
}
