package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"streetlight/internal/apperr"
	"streetlight/pkg/pagination"
)

// partSortSpec 定义备件台账允许的排序字段白名单。
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

// txnSortSpec 定义出入库流水允许的排序字段白名单。
var txnSortSpec = pagination.SortSpec{
	Allowed: map[string]string{
		"tx_no":       "tx_no",
		"occurred_at": "occurred_at",
		"tx_type":     "tx_type",
		"part_code":   "part_code",
		"quantity":    "quantity",
		"created_at":  "created_at",
	},
	Default: "occurred_at",
}

// ShortageError 库存不足冲突, 除错误文案外携带每种备件的可用数量, 供前端展示与补货。
type ShortageError struct {
	AppError *apperr.Error
	Details  []ShortageDetail
}

// Error 实现 error 接口。
func (e *ShortageError) Error() string { return e.AppError.Error() }

// Unwrap 支持 errors.As 提取底层业务错误的状态码与错误码。
func (e *ShortageError) Unwrap() error { return e.AppError }

// ErrorDetails 实现统一响应的 details 载体, 会随 409 响应体返回。
func (e *ShortageError) ErrorDetails() any { return e.Details }

// NewShortageError 依据缺货明细生成冲突错误, 文案中直接给出可用数量。
func NewShortageError(details []ShortageDetail) *ShortageError {
	parts := make([]string, 0, len(details))
	for _, item := range details {
		parts = append(parts, fmt.Sprintf("%s(%s) 需 %d %s, 当前可用 %d %s",
			item.PartName, item.PartCode, item.Need, item.Unit, item.Available, item.Unit))
	}
	return &ShortageError{
		AppError: apperr.Conflict("备件库存不足, 无法开工: %s", strings.Join(parts, "; ")),
		Details:  details,
	}
}

// Service 承载备件库存的业务规则, 并向维修模块提供领用/退料/回退能力。
type Service struct {
	repo *Repository
}

// NewService 构造备件库存服务。
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ---- 备件台账 ----

// GetPart 查询备件详情。
func (s *Service) GetPart(ctx context.Context, id uint) (*SparePart, error) {
	return s.repo.GetPartByID(ctx, id)
}

// ListParts 分页查询备件台账。
func (s *Service) ListParts(ctx context.Context, query PartListQuery) ([]SparePart, int64, pagination.Query, error) {
	page := pagination.Parse(query.Params, partSortSpec)
	filter := PartFilter{
		Keyword:  strings.TrimSpace(query.Keyword),
		Category: strings.TrimSpace(query.Category),
		Status:   strings.TrimSpace(query.Status),
		Shortage: query.Shortage,
	}
	if filter.Status != "" && !IsValidPartStatus(filter.Status) {
		return nil, 0, page, apperr.BadRequest("非法的备件状态: %s", filter.Status)
	}
	items, total, err := s.repo.ListParts(ctx, filter, page)
	if err != nil {
		return nil, 0, page, err
	}
	return items, total, page, nil
}

// CreatePart 新增备件档案, initial_stock > 0 时同步写入一条期初入库流水。
func (s *Service) CreatePart(ctx context.Context, req PartCreateRequest) (*SparePart, error) {
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, apperr.BadRequest("备件编号不能为空")
	}
	exists, err := s.repo.GetPartByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if exists != nil {
		return nil, apperr.Conflict("备件编号已存在: %s", code)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.BadRequest("备件名称不能为空")
	}
	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		unit = "个"
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = PartStatusActive
	}
	initialStock := 0
	if req.InitialStock != nil {
		initialStock = *req.InitialStock
	}
	safetyStock := 0
	if req.SafetyStock != nil {
		safetyStock = *req.SafetyStock
	}
	unitPrice := 0.0
	if req.UnitPrice != nil {
		unitPrice = *req.UnitPrice
	}

	part := &SparePart{
		Code:        code,
		Name:        name,
		Category:    strings.TrimSpace(req.Category),
		Spec:        strings.TrimSpace(req.Spec),
		Unit:        unit,
		Stock:       initialStock,
		SafetyStock: safetyStock,
		UnitPrice:   unitPrice,
		Supplier:    strings.TrimSpace(req.Supplier),
		Status:      status,
		Remark:      strings.TrimSpace(req.Remark),
	}

	err = s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(part).Error; err != nil {
			return fmt.Errorf("新增备件失败: %w", err)
		}
		if initialStock > 0 {
			txn := buildTxn(TxInbound, part, initialStock, initialStock, 0, initialStock,
				unitPrice, "期初库存", nil, "", "", time.Now())
			if err := s.repo.CreateTransactionWithUniqueNo(tx, txn, txnPrefix(TxInbound, time.Now())); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return part, nil
}

// UpdatePart 修改备件档案, 库存数量不允许通过编辑直接调整。
func (s *Service) UpdatePart(ctx context.Context, id uint, req PartUpdateRequest) (*SparePart, error) {
	part, err := s.repo.GetPartByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperr.BadRequest("备件名称不能为空")
		}
		part.Name = name
	}
	if req.Category != nil {
		part.Category = strings.TrimSpace(*req.Category)
	}
	if req.Spec != nil {
		part.Spec = strings.TrimSpace(*req.Spec)
	}
	if req.Unit != nil {
		unit := strings.TrimSpace(*req.Unit)
		if unit == "" {
			return nil, apperr.BadRequest("计量单位不能为空")
		}
		part.Unit = unit
	}
	if req.SafetyStock != nil {
		if *req.SafetyStock < 0 {
			return nil, apperr.BadRequest("安全库存不能小于 0")
		}
		part.SafetyStock = *req.SafetyStock
	}
	if req.UnitPrice != nil {
		if *req.UnitPrice < 0 {
			return nil, apperr.BadRequest("单价不能小于 0")
		}
		part.UnitPrice = *req.UnitPrice
	}
	if req.Supplier != nil {
		part.Supplier = strings.TrimSpace(*req.Supplier)
	}
	if req.Status != nil {
		if !IsValidPartStatus(*req.Status) {
			return nil, apperr.BadRequest("非法的备件状态: %s", *req.Status)
		}
		part.Status = *req.Status
	}
	if req.Remark != nil {
		part.Remark = strings.TrimSpace(*req.Remark)
	}
	if err := s.repo.UpdatePart(ctx, part); err != nil {
		return nil, err
	}
	return part, nil
}

// DeletePart 删除备件, 存在出入库流水或仍有库存时拒绝, 保证台账可追溯。
func (s *Service) DeletePart(ctx context.Context, id uint) error {
	part, err := s.repo.GetPartByID(ctx, id)
	if err != nil {
		return err
	}
	if part.Stock > 0 {
		return apperr.Conflict("备件 %s 仍有 %d %s 库存, 请先报废清零后再删除", part.Code, part.Stock, part.Unit)
	}
	txnCount, err := s.repo.CountPartTransactions(ctx, id)
	if err != nil {
		return err
	}
	if txnCount > 0 {
		return apperr.Conflict("备件 %s 已存在 %d 条出入库流水, 为保证台账可追溯不允许删除, 可改为停用", part.Code, txnCount)
	}
	return s.repo.DeletePart(ctx, id)
}

// Inbound 采购(或盘盈)入库, 按移动加权平均法刷新备件单价。
func (s *Service) Inbound(ctx context.Context, partID uint, req InboundRequest) (*StockTransaction, error) {
	if req.Quantity <= 0 {
		return nil, apperr.BadRequest("入库数量必须大于 0")
	}
	occurredAt, err := parseTime(req.OccurredAt, time.Now())
	if err != nil {
		return nil, err
	}
	unitPrice := 0.0
	if req.UnitPrice != nil {
		unitPrice = *req.UnitPrice
	}

	var txn *StockTransaction
	err = s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		part, err := s.repo.GetPartByIDForUpdate(tx, partID)
		if err != nil {
			return err
		}
		before := part.Stock
		after := before + req.Quantity
		if req.UnitPrice != nil {
			part.UnitPrice = (float64(before)*part.UnitPrice + float64(req.Quantity)*unitPrice) / float64(after)
		}
		supplier := strings.TrimSpace(req.Supplier)
		if supplier != "" {
			part.Supplier = supplier
		}
		part.Stock = after
		if err := tx.Save(part).Error; err != nil {
			return fmt.Errorf("更新备件库存失败: %w", err)
		}

		txn = buildTxn(TxInbound, part, req.Quantity, req.Quantity, before, after,
			unitPrice, strings.TrimSpace(req.Reason), nil, "", strings.TrimSpace(req.Operator), occurredAt)
		return s.repo.CreateTransactionWithUniqueNo(tx, txn, txnPrefix(TxInbound, occurredAt))
	})
	if err != nil {
		return nil, err
	}
	return txn, nil
}

// ScrapPart 从库存直接报废备件, 必须登记报废原因, 且数量不能超过当前库存。
func (s *Service) ScrapPart(ctx context.Context, partID uint, req PartScrapRequest) (*StockTransaction, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, apperr.BadRequest("报废原因不能为空")
	}
	if req.Quantity <= 0 {
		return nil, apperr.BadRequest("报废数量必须大于 0")
	}
	occurredAt, err := parseTime(req.OccurredAt, time.Now())
	if err != nil {
		return nil, err
	}

	var txn *StockTransaction
	err = s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		part, err := s.repo.GetPartByIDForUpdate(tx, partID)
		if err != nil {
			return err
		}
		if req.Quantity > part.Stock {
			return apperr.BadRequest("报废数量 %d %s 超过当前库存 %d %s", req.Quantity, part.Unit, part.Stock, part.Unit)
		}
		before := part.Stock
		after := before - req.Quantity
		part.Stock = after
		if err := tx.Save(part).Error; err != nil {
			return fmt.Errorf("更新备件库存失败: %w", err)
		}
		txn = buildTxn(TxScrap, part, req.Quantity, -req.Quantity, before, after,
			part.UnitPrice, reason, nil, "", strings.TrimSpace(req.Operator), occurredAt)
		return s.repo.CreateTransactionWithUniqueNo(tx, txn, txnPrefix(TxScrap, occurredAt))
	})
	if err != nil {
		return nil, err
	}
	return txn, nil
}

// ---- 出入库流水与选项 ----

// ListTransactions 分页查询出入库流水。
func (s *Service) ListTransactions(ctx context.Context, query TxnListQuery) ([]StockTransaction, int64, pagination.Query, error) {
	page := pagination.Parse(query.Params, txnSortSpec)
	filter := TxnFilter{
		Keyword:  strings.TrimSpace(query.Keyword),
		TxType:   strings.TrimSpace(query.TxType),
		PartID:   query.PartID,
		RepairNo: strings.TrimSpace(query.RepairNo),
		FaultNo:  strings.TrimSpace(query.FaultNo),
		Operator: strings.TrimSpace(query.Operator),
	}
	if filter.TxType != "" && !IsValidTxType(filter.TxType) {
		return nil, 0, page, apperr.BadRequest("非法的出入库类型: %s", filter.TxType)
	}
	if value := strings.TrimSpace(query.StartDate); value != "" {
		from, err := parseDay(value)
		if err != nil {
			return nil, 0, page, err
		}
		filter.From = &from
	}
	if value := strings.TrimSpace(query.EndDate); value != "" {
		to, err := parseDay(value)
		if err != nil {
			return nil, 0, page, err
		}
		to = to.AddDate(0, 0, 1)
		filter.To = &to
	}
	if filter.From != nil && filter.To != nil && filter.To.Before(*filter.From) {
		return nil, 0, page, apperr.BadRequest("结束日期不能早于开始日期")
	}
	items, total, err := s.repo.ListTransactions(ctx, filter, page)
	if err != nil {
		return nil, 0, page, err
	}
	return items, total, page, nil
}

// Options 返回备件下拉选项(含实时可用库存)与建议编号。
func (s *Service) Options(ctx context.Context) (*PartOptions, error) {
	filter := PartFilter{}
	page := pagination.Query{Page: 1, PageSize: pagination.MaxPageSize, SortColumn: "id"}
	parts, _, err := s.repo.ListParts(ctx, filter, page)
	if err != nil {
		return nil, err
	}
	categories, err := s.repo.DistinctPartValues(ctx, "category")
	if err != nil {
		return nil, err
	}
	units, err := s.repo.DistinctPartValues(ctx, "unit")
	if err != nil {
		return nil, err
	}
	nextCode, err := s.repo.NextPartCode(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]PartOption, 0, len(parts))
	for _, part := range parts {
		items = append(items, PartOption{
			ID:          part.ID,
			Code:        part.Code,
			Name:        part.Name,
			Spec:        part.Spec,
			Unit:        part.Unit,
			Stock:       part.Stock,
			SafetyStock: part.SafetyStock,
			UnitPrice:   part.UnitPrice,
			Status:      part.Status,
		})
	}
	return &PartOptions{Items: items, Categories: categories, Units: units, NextCode: nextCode}, nil
}

// Metadata 返回库存模块字典。
func (s *Service) Metadata() *Meta {
	return &Meta{TxTypes: TxTypes(), PartStatuses: PartStatuses()}
}

// Statistics 汇总备件库存统计。
func (s *Service) Statistics(ctx context.Context) (*PartStatistics, error) {
	total, byStatus, err := s.repo.CountParts(ctx)
	if err != nil {
		return nil, err
	}
	shortage, err := s.repo.CountShortage(ctx)
	if err != nil {
		return nil, err
	}
	qty, value, err := s.repo.StockTotals(ctx)
	if err != nil {
		return nil, err
	}
	return &PartStatistics{
		Total:           total,
		ActiveTotal:     byStatus[PartStatusActive],
		InactiveTotal:   byStatus[PartStatusInactive],
		ShortageTotal:   shortage,
		TotalStockQty:   qty,
		TotalStockValue: value,
	}, nil
}

// ConsumptionRanking 返回备件净消耗排名。
func (s *Service) ConsumptionRanking(ctx context.Context, limit int) ([]ConsumptionRow, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.ConsumptionRanking(ctx, limit)
}

// ShortageList 返回缺货预警清单。
func (s *Service) ShortageList(ctx context.Context, limit int) ([]ShortageItem, error) {
	if limit <= 0 {
		limit = 20
	}
	parts, err := s.repo.ListShortage(ctx, limit)
	if err != nil {
		return nil, err
	}
	items := make([]ShortageItem, 0, len(parts))
	for _, part := range parts {
		gap := part.SafetyStock + 1 - part.Stock
		if gap < 1 {
			gap = 1
		}
		items = append(items, ShortageItem{
			ID: part.ID, Code: part.Code, Name: part.Name, Spec: part.Spec, Unit: part.Unit,
			Stock: part.Stock, SafetyStock: part.SafetyStock, Gap: gap,
		})
	}
	return items, nil
}

// ---- 维修联动 ----

// CheckAvailable 开工前的库存可用性预检, 不落任何数据; 不足时返回携带可用数量的冲突错误。
func (s *Service) CheckAvailable(ctx context.Context, lines []MaterialLine) error {
	normalized, err := normalizeLines(lines)
	if err != nil {
		return err
	}
	ids := make([]uint, 0, len(normalized))
	for _, line := range normalized {
		ids = append(ids, line.PartID)
	}
	parts, err := s.repo.ListPartsByIDs(ctx, ids)
	if err != nil {
		return err
	}
	partMap := make(map[uint]SparePart, len(parts))
	for _, part := range parts {
		partMap[part.ID] = part
	}
	shortages := make([]ShortageDetail, 0)
	for _, line := range normalized {
		part, ok := partMap[line.PartID]
		if !ok {
			return apperr.NotFound("备件不存在: id=%d", line.PartID)
		}
		available := 0
		if part.Status == PartStatusActive {
			available = part.Stock
		}
		if available < line.Quantity {
			shortages = append(shortages, ShortageDetail{
				PartID: part.ID, PartCode: part.Code, PartName: part.Name, Unit: part.Unit,
				Need: line.Quantity, Available: available,
			})
		}
	}
	if len(shortages) > 0 {
		return NewShortageError(shortages)
	}
	return nil
}

// ReserveMaterials 维修开工: 在一个事务内锁定备件、校验库存、扣减并写入领用流水与用料明细。
func (s *Service) ReserveMaterials(ctx context.Context, lines []MaterialLine, ref RepairRef) ([]RepairMaterial, error) {
	normalized, err := normalizeLines(lines)
	if err != nil {
		return nil, err
	}
	if ref.ID == 0 {
		return nil, apperr.BadRequest("维修单信息缺失, 无法领用备件")
	}

	materials := make([]RepairMaterial, 0, len(normalized))
	err = s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		// 按备件 ID 排序后加锁, 避免并发事务交叉持锁导致死锁。
		sort.Slice(normalized, func(i, j int) bool { return normalized[i].PartID < normalized[j].PartID })
		locked := make([]SparePart, 0, len(normalized))
		shortages := make([]ShortageDetail, 0)
		for _, line := range normalized {
			part, err := s.repo.GetPartByIDForUpdate(tx, line.PartID)
			if err != nil {
				return err
			}
			locked = append(locked, *part)
			available := 0
			if part.Status == PartStatusActive {
				available = part.Stock
			}
			if available < line.Quantity {
				shortages = append(shortages, ShortageDetail{
					PartID: part.ID, PartCode: part.Code, PartName: part.Name, Unit: part.Unit,
					Need: line.Quantity, Available: available,
				})
			}
		}
		if len(shortages) > 0 {
			return NewShortageError(shortages)
		}

		for index, line := range normalized {
			part := locked[index]
			before := part.Stock
			after := before - line.Quantity
			part.Stock = after
			if err := tx.Save(&part).Error; err != nil {
				return fmt.Errorf("扣减备件库存失败: %w", err)
			}

			repairID := ref.ID
			txn := buildTxn(TxIssue, &part, line.Quantity, -line.Quantity, before, after,
				part.UnitPrice, "维修开工领用", &repairID, ref.FaultNo, ref.Operator, ref.OccurredAt)
			txn.RepairNo = ref.RepairNo
			if err := s.repo.CreateTransactionWithUniqueNo(tx, txn, txnPrefix(TxIssue, ref.OccurredAt)); err != nil {
				return err
			}

			materials = append(materials, RepairMaterial{
				RepairID:  ref.ID,
				RepairNo:  ref.RepairNo,
				FaultID:   ref.FaultID,
				FaultNo:   ref.FaultNo,
				LampID:    ref.LampID,
				LampCode:  ref.LampCode,
				PartID:    part.ID,
				PartCode:  part.Code,
				PartName:  part.Name,
				Spec:      part.Spec,
				Unit:      part.Unit,
				Quantity:  line.Quantity,
				UnitPrice: part.UnitPrice,
				Subtotal:  part.UnitPrice * float64(line.Quantity),
			})
		}
		return s.repo.CreateMaterials(tx, materials)
	})
	if err != nil {
		return nil, err
	}
	return materials, nil
}

// ReturnMaterial 维修退料: 把未使用的备件退回可用库存, 必须登记退料原因。
func (s *Service) ReturnMaterial(ctx context.Context, materialID uint, req MaterialReturnRequest) (*StockTransaction, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, apperr.BadRequest("退料原因不能为空")
	}
	if req.Quantity <= 0 {
		return nil, apperr.BadRequest("退料数量必须大于 0")
	}

	var txn *StockTransaction
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		material, err := s.repo.GetMaterialByIDForUpdate(tx, materialID)
		if err != nil {
			return err
		}
		openQty := material.OpenQty()
		if req.Quantity > openQty {
			return apperr.BadRequest("退料数量 %d %s 超过该维修单可退数量 %d %s",
				req.Quantity, material.Unit, openQty, material.Unit)
		}
		part, err := s.repo.GetPartByIDForUpdate(tx, material.PartID)
		if err != nil {
			return err
		}
		before := part.Stock
		after := before + req.Quantity
		part.Stock = after
		if err := tx.Save(part).Error; err != nil {
			return fmt.Errorf("退回备件库存失败: %w", err)
		}
		material.ReturnedQty += req.Quantity
		if err := tx.Save(material).Error; err != nil {
			return fmt.Errorf("更新维修用料明细失败: %w", err)
		}

		repairID := material.RepairID
		txn = buildTxn(TxReturn, part, req.Quantity, req.Quantity, before, after,
			part.UnitPrice, reason, &repairID, material.FaultNo, strings.TrimSpace(req.Operator), time.Now())
		txn.RepairNo = material.RepairNo
		return s.repo.CreateTransactionWithUniqueNo(tx, txn, txnPrefix(TxReturn, time.Now()))
	})
	if err != nil {
		return nil, err
	}
	return txn, nil
}

// ScrapMaterial 维修现场报废: 已领用但未退库的备件损坏/灭失时登记原因, 库存不再变动。
func (s *Service) ScrapMaterial(ctx context.Context, materialID uint, req MaterialScrapRequest) (*StockTransaction, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, apperr.BadRequest("报废原因不能为空")
	}
	if req.Quantity <= 0 {
		return nil, apperr.BadRequest("报废数量必须大于 0")
	}

	var txn *StockTransaction
	err := s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		material, err := s.repo.GetMaterialByIDForUpdate(tx, materialID)
		if err != nil {
			return err
		}
		openQty := material.OpenQty()
		if req.Quantity > openQty {
			return apperr.BadRequest("报废数量 %d %s 超过该维修单未结数量 %d %s",
				req.Quantity, material.Unit, openQty, material.Unit)
		}
		part, err := s.repo.GetPartByIDForUpdate(tx, material.PartID)
		if err != nil {
			return err
		}
		current := part.Stock
		material.ScrappedQty += req.Quantity
		if err := tx.Save(material).Error; err != nil {
			return fmt.Errorf("更新维修用料明细失败: %w", err)
		}

		// 备件开工时已出库, 现场报废只做审计留痕, 库存增量为 0。
		repairID := material.RepairID
		txn = buildTxn(TxScrap, part, req.Quantity, 0, current, current,
			material.UnitPrice, reason, &repairID, material.FaultNo, strings.TrimSpace(req.Operator), time.Now())
		txn.RepairNo = material.RepairNo
		return s.repo.CreateTransactionWithUniqueNo(tx, txn, txnPrefix(TxScrap, time.Now()))
	})
	if err != nil {
		return nil, err
	}
	return txn, nil
}

// RollbackRepair 删除维修记录时把仍挂账的领用备件全部回退入库, 并清理用料明细。
// 已退料(已回库)与已现场报废(已消耗)的数量不再处理。
func (s *Service) RollbackRepair(ctx context.Context, repairID uint) error {
	materials, err := s.repo.ListMaterialsByRepair(ctx, repairID)
	if err != nil {
		return err
	}
	if len(materials) == 0 {
		return nil
	}
	return s.repo.Transaction(ctx, func(tx *gorm.DB) error {
		// 按备件聚合挂账数量, 同一备件在一张维修单上只会有一行, 这里仍做防御性合并。
		byPart := make(map[uint]int)
		for index := range materials {
			material := &materials[index]
			if open := material.OpenQty(); open > 0 {
				byPart[material.PartID] += open
			}
		}
		partIDs := make([]uint, 0, len(byPart))
		for partID := range byPart {
			partIDs = append(partIDs, partID)
		}
		sort.Slice(partIDs, func(i, j int) bool { return partIDs[i] < partIDs[j] })

		for _, partID := range partIDs {
			openQty := byPart[partID]
			part, err := s.repo.GetPartByIDForUpdate(tx, partID)
			if err != nil {
				return err
			}
			before := part.Stock
			after := before + openQty
			part.Stock = after
			if err := tx.Save(part).Error; err != nil {
				return fmt.Errorf("回退备件库存失败: %w", err)
			}
			var sample *RepairMaterial
			for index := range materials {
				if materials[index].PartID == partID {
					sample = &materials[index]
					break
				}
			}
			repairIDValue := repairID
			txn := buildTxn(TxRollback, part, openQty, openQty, before, after,
				part.UnitPrice, "删除维修记录自动回退", &repairIDValue, sample.FaultNo, "系统", time.Now())
			txn.RepairNo = sample.RepairNo
			if err := s.repo.CreateTransactionWithUniqueNo(tx, txn, txnPrefix(TxRollback, time.Now())); err != nil {
				return err
			}
		}
		return s.repo.DeleteMaterialsByRepair(tx, repairID)
	})
}

// ListMaterialsByRepair 查询某次维修的用料明细。
func (s *Service) ListMaterialsByRepair(ctx context.Context, repairID uint) ([]RepairMaterial, error) {
	return s.repo.ListMaterialsByRepair(ctx, repairID)
}

// ListMaterialsByRepairs 批量查询多次维修的用料明细, 供只读视图组装。
func (s *Service) ListMaterialsByRepairs(ctx context.Context, repairIDs []uint) (map[uint][]RepairMaterial, error) {
	return s.repo.ListMaterialsByRepairs(ctx, repairIDs)
}

// ---- 内部辅助 ----

// normalizeLines 校验并合并同备件的领用行, 不允许数量小于等于 0。
func normalizeLines(lines []MaterialLine) ([]MaterialLine, error) {
	if len(lines) == 0 {
		return nil, nil
	}
	merged := make(map[uint]int, len(lines))
	order := make([]uint, 0, len(lines))
	for _, line := range lines {
		if line.PartID == 0 {
			return nil, apperr.BadRequest("领用备件不能为空")
		}
		if line.Quantity <= 0 {
			return nil, apperr.BadRequest("备件 %d 的领用数量必须大于 0", line.PartID)
		}
		if _, exists := merged[line.PartID]; !exists {
			order = append(order, line.PartID)
		}
		merged[line.PartID] += line.Quantity
	}
	result := make([]MaterialLine, 0, len(order))
	for _, partID := range order {
		result = append(result, MaterialLine{PartID: partID, Quantity: merged[partID]})
	}
	return result, nil
}

// buildTxn 组装一条出入库流水。
func buildTxn(
	txType string, part *SparePart, quantity, delta, before, after int,
	unitPrice float64, reason string, repairID *uint, faultNo, operator string, occurredAt time.Time,
) *StockTransaction {
	return &StockTransaction{
		TxType:      txType,
		PartID:      part.ID,
		PartCode:    part.Code,
		PartName:    part.Name,
		Spec:        part.Spec,
		Unit:        part.Unit,
		Quantity:    quantity,
		StockDelta:  delta,
		StockBefore: before,
		StockAfter:  after,
		UnitPrice:   unitPrice,
		Reason:      reason,
		RepairID:    repairID,
		FaultNo:     faultNo,
		Operator:    operator,
		OccurredAt:  occurredAt,
	}
}

// txnPrefix 依据流水类型与时间生成单号前缀, 例如 LY20260919。
func txnPrefix(txType string, at time.Time) string {
	return txnTypePrefix(txType) + at.Format("20060102")
}

func txnTypePrefix(txType string) string {
	switch txType {
	case TxInbound:
		return "RK"
	case TxIssue:
		return "LY"
	case TxReturn:
		return "TL"
	case TxScrap:
		return "BF"
	case TxRollback:
		return "HC"
	case TxAdjustOut:
		return "PK"
	default:
		return "KC"
	}
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
