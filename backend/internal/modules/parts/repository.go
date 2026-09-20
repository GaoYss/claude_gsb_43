package parts

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"streetlight/internal/apperr"
	"streetlight/pkg/pagination"
)

// Filter 是仓储层使用的备件台账查询条件。
type Filter struct {
	Keyword  string
	Category string
	Shortage string
}

// TxFilter 是库存流水查询条件。
type TxFilter struct {
	Keyword      string
	PartID       uint
	Type         string
	Operator     string
	OccurredFrom *time.Time
	OccurredTo   *time.Time
}

// Repository 负责备件台账、库存流水与维修用料明细的数据访问。
type Repository struct {
	db *gorm.DB
}

// NewRepository 构造备件仓储。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) session(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

// Transaction 在一个数据库事务中执行 fn, fn 内统一使用返回的 tx。
func (r *Repository) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.session(ctx).Transaction(fn)
}

// ---- 备件台账 ----

// CreatePart 新增备件。
func (r *Repository) CreatePart(ctx context.Context, entity *Part) error {
	if err := r.session(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("新增备件失败: %w", err)
	}
	return nil
}

// CreatePartTx 在给定事务内新增备件。
func (r *Repository) CreatePartTx(tx *gorm.DB, entity *Part) error {
	if err := tx.Create(entity).Error; err != nil {
		return fmt.Errorf("新增备件失败: %w", err)
	}
	return nil
}

// UpdatePart 保存备件全部字段。
func (r *Repository) UpdatePart(ctx context.Context, entity *Part) error {
	if err := r.session(ctx).Save(entity).Error; err != nil {
		return fmt.Errorf("更新备件失败: %w", err)
	}
	return nil
}

// DeletePart 按主键删除备件, 存在库存流水或用料明细时数据库外键/业务会阻止。
func (r *Repository) DeletePart(ctx context.Context, id uint) error {
	if err := r.session(ctx).Delete(&Part{}, id).Error; err != nil {
		return fmt.Errorf("删除备件失败: %w", err)
	}
	return nil
}

// CountTx 统计备件相关流水/明细数量, 供删除前校验。
func (r *Repository) CountStockTxByPart(ctx context.Context, partID uint) (int64, error) {
	var count int64
	if err := r.session(ctx).Model(&StockTx{}).Where("part_id = ?", partID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计备件流水失败: %w", err)
	}
	return count, nil
}

// CountMaterialByPart 统计备件被维修领用的明细条数。
func (r *Repository) CountMaterialByPart(ctx context.Context, partID uint) (int64, error) {
	var count int64
	if err := r.session(ctx).Model(&RepairMaterial{}).Where("part_id = ?", partID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计备件用料明细失败: %w", err)
	}
	return count, nil
}

// GetPartByID 按主键查询备件。
func (r *Repository) GetPartByID(ctx context.Context, id uint) (*Part, error) {
	var entity Part
	err := r.session(ctx).First(&entity, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("备件不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询备件失败: %w", err)
	}
	return &entity, nil
}

// GetPartByIDTx 在事务内按主键查询备件并加行锁(数据库不支持时退化为普通查询)。
func (r *Repository) GetPartByIDTx(tx *gorm.DB, id uint) (*Part, error) {
	var entity Part
	err := tx.First(&entity, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("备件不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询备件失败: %w", err)
	}
	return &entity, nil
}

// ExistsPartByCode 判断备件编号是否已被占用。
func (r *Repository) ExistsPartByCode(ctx context.Context, code string, excludeID uint) (bool, error) {
	query := r.session(ctx).Model(&Part{}).Where("code = ?", strings.TrimSpace(code))
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("校验备件编号失败: %w", err)
	}
	return count > 0, nil
}

// ListParts 分页查询备件台账。
func (r *Repository) ListParts(ctx context.Context, filter Filter, page pagination.Query) ([]Part, int64, error) {
	base := func() *gorm.DB {
		return applyPartFilter(r.session(ctx).Model(&Part{}), filter)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计备件总数失败: %w", err)
	}

	entities := make([]Part, 0)
	if err := base().Order(page.OrderClause()).Offset(page.Offset()).Limit(page.Limit()).Find(&entities).Error; err != nil {
		return nil, 0, fmt.Errorf("查询备件列表失败: %w", err)
	}
	return entities, total, nil
}

// ListPartByIDsTx 在事务内按主键批量查询备件。
func (r *Repository) ListPartByIDsTx(tx *gorm.DB, ids []uint) ([]Part, error) {
	entities := make([]Part, 0, len(ids))
	if len(ids) == 0 {
		return entities, nil
	}
	if err := tx.Where("id IN ?", ids).Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("批量查询备件失败: %w", err)
	}
	return entities, nil
}

// UpdateStockTx 在事务内更新备件库存。
func (r *Repository) UpdateStockTx(tx *gorm.DB, id uint, stock int) error {
	result := tx.Model(&Part{}).Where("id = ?", id).Update("stock", stock)
	if result.Error != nil {
		return fmt.Errorf("更新备件库存失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return apperr.NotFound("备件不存在: id=%d", id)
	}
	return nil
}

// CountParts 统计备件种类总数。
func (r *Repository) CountParts(ctx context.Context) (int64, error) {
	var total int64
	if err := r.session(ctx).Model(&Part{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("统计备件种类失败: %w", err)
	}
	return total, nil
}

// CountPartsByColumn 按列分组统计。
func (r *Repository) CountPartsByColumn(ctx context.Context, column string) (map[string]int64, error) {
	type row struct {
		Label string
		Total int64
	}
	rows := make([]row, 0)
	err := r.session(ctx).Model(&Part{}).
		Select(column + " AS label, COUNT(*) AS total").
		Group(column).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("分组统计 %s 失败: %w", column, err)
	}
	result := make(map[string]int64, len(rows))
	for _, item := range rows {
		result[item.Label] = item.Total
	}
	return result, nil
}

// SumPartStock 汇总库存数量与库存金额。
func (r *Repository) SumPartStock(ctx context.Context) (int64, float64, error) {
	type row struct {
		TotalStock int64
		TotalValue float64
	}
	var result row
	err := r.session(ctx).Model(&Part{}).
		Select("COALESCE(SUM(stock), 0) AS total_stock, COALESCE(SUM(stock * unit_price), 0) AS total_value").
		Scan(&result).Error
	if err != nil {
		return 0, 0, fmt.Errorf("汇总库存失败: %w", err)
	}
	return result.TotalStock, result.TotalValue, nil
}

// CountShortage 统计缺货(库存 <= 安全库存)与零库存的备件种类。
func (r *Repository) CountShortage(ctx context.Context) (int64, int64, error) {
	var shortage, out int64
	if err := r.session(ctx).Model(&Part{}).Where("stock <= safety_stock").Count(&shortage).Error; err != nil {
		return 0, 0, fmt.Errorf("统计缺货备件失败: %w", err)
	}
	if err := r.session(ctx).Model(&Part{}).Where("stock = 0").Count(&out).Error; err != nil {
		return 0, 0, fmt.Errorf("统计零库存备件失败: %w", err)
	}
	return shortage, out, nil
}

// ListShortage 返回缺货/预警备件清单, 缺货缺口大的排在前面。
func (r *Repository) ListShortage(ctx context.Context, limit int) ([]Part, error) {
	entities := make([]Part, 0)
	query := r.session(ctx).Where("stock <= safety_stock").
		Order("safety_stock - stock DESC, stock ASC, id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("查询缺货备件失败: %w", err)
	}
	return entities, nil
}

// DistinctPartValues 返回备件某列的去重取值。
func (r *Repository) DistinctPartValues(ctx context.Context, column string) ([]string, error) {
	values := make([]string, 0)
	err := r.session(ctx).Model(&Part{}).
		Where(column+" <> ''").
		Distinct().
		Order(column).
		Pluck(column, &values).Error
	if err != nil {
		return nil, fmt.Errorf("查询 %s 选项失败: %w", column, err)
	}
	return values, nil
}

// NextPartCode 依据当前最大编号推算下一个建议编号, 例如 BJ-000008。
func (r *Repository) NextPartCode(ctx context.Context) (string, error) {
	var latest string
	if err := r.session(ctx).Model(&Part{}).Order("id DESC").Limit(1).Pluck("code", &latest).Error; err != nil {
		return "", fmt.Errorf("生成备件编号失败: %w", err)
	}
	next := 1
	if index := strings.LastIndex(latest, "-"); index >= 0 {
		if value, convErr := strconv.Atoi(latest[index+1:]); convErr == nil {
			next = value + 1
		}
	}
	return fmt.Sprintf("BJ-%05d", next), nil
}

// ---- 库存流水 ----

// CreateStockTx 落库一条库存流水。
func (r *Repository) CreateStockTx(tx *gorm.DB, entity *StockTx) error {
	if err := tx.Create(entity).Error; err != nil {
		return fmt.Errorf("登记库存流水失败: %w", err)
	}
	return nil
}

// NextTxSequence 返回指定前缀下可用的下一个流水号序号。
func (r *Repository) NextTxSequence(ctx context.Context, prefix string) (int, error) {
	return nextTxSequence(r.session(ctx), prefix)
}

// NextTxSequenceTx 在事务内返回下一个流水号序号。
func (r *Repository) NextTxSequenceTx(tx *gorm.DB, prefix string) (int, error) {
	return nextTxSequence(tx, prefix)
}

func nextTxSequence(statement *gorm.DB, prefix string) (int, error) {
	var latest string
	err := statement.Model(&StockTx{}).
		Where("tx_no LIKE ?", prefix+"%").
		Order("tx_no DESC").
		Limit(1).
		Pluck("tx_no", &latest).Error
	if err != nil {
		return 0, fmt.Errorf("生成库存流水号失败: %w", err)
	}
	if latest == "" {
		return 1, nil
	}
	value, convErr := strconv.Atoi(strings.TrimPrefix(latest, prefix))
	if convErr != nil {
		return 1, nil
	}
	return value + 1, nil
}

// ListStockTx 分页查询库存流水。
func (r *Repository) ListStockTx(ctx context.Context, filter TxFilter, page pagination.Query) ([]StockTx, int64, error) {
	base := func() *gorm.DB {
		return applyTxFilter(r.session(ctx).Model(&StockTx{}), filter)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计库存流水失败: %w", err)
	}

	entities := make([]StockTx, 0)
	if err := base().Order(page.OrderClause()).Offset(page.Offset()).Limit(page.Limit()).Find(&entities).Error; err != nil {
		return nil, 0, fmt.Errorf("查询库存流水失败: %w", err)
	}
	return entities, total, nil
}

// ---- 维修用料明细 ----

// CreateMaterialsTx 在事务内批量写入维修用料明细。
func (r *Repository) CreateMaterialsTx(tx *gorm.DB, items []RepairMaterial) error {
	if len(items) == 0 {
		return nil
	}
	if err := tx.Create(&items).Error; err != nil {
		return fmt.Errorf("写入维修用料明细失败: %w", err)
	}
	return nil
}

// ListMaterialsByRepair 查询某次维修的用料明细。
func (r *Repository) ListMaterialsByRepair(ctx context.Context, repairID uint) ([]RepairMaterial, error) {
	return listMaterialsByRepair(r.session(ctx), repairID)
}

// ListMaterialsByRepairTx 在事务内查询某次维修的用料明细。
func (r *Repository) ListMaterialsByRepairTx(tx *gorm.DB, repairID uint) ([]RepairMaterial, error) {
	return listMaterialsByRepair(tx, repairID)
}

func listMaterialsByRepair(statement *gorm.DB, repairID uint) ([]RepairMaterial, error) {
	items := make([]RepairMaterial, 0)
	if err := statement.Where("repair_id = ?", repairID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("查询维修用料明细失败: %w", err)
	}
	return items, nil
}

// ListMaterialsByRepairs 批量查询多次维修的用料明细。
func (r *Repository) ListMaterialsByRepairs(ctx context.Context, repairIDs []uint) ([]RepairMaterial, error) {
	items := make([]RepairMaterial, 0)
	if len(repairIDs) == 0 {
		return items, nil
	}
	if err := r.session(ctx).Where("repair_id IN ?", repairIDs).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("批量查询维修用料明细失败: %w", err)
	}
	return items, nil
}

// DeleteMaterialsByRepairTx 在事务内删除某次维修的用料明细。
func (r *Repository) DeleteMaterialsByRepairTx(tx *gorm.DB, repairID uint) error {
	if err := tx.Where("repair_id = ?", repairID).Delete(&RepairMaterial{}).Error; err != nil {
		return fmt.Errorf("删除维修用料明细失败: %w", err)
	}
	return nil
}

// SumReturnedQuantity 统计某维修单下某备件已退料数量(退料数量不能超过领用量)。
func (r *Repository) SumReturnedQuantity(ctx context.Context, repairID uint, partID uint) (int, error) {
	return sumReturnedQuantity(r.session(ctx), repairID, partID)
}

// SumReturnedQuantityTx 在事务内统计某维修单下某备件已退料数量。
func (r *Repository) SumReturnedQuantityTx(tx *gorm.DB, repairID uint, partID uint) (int, error) {
	return sumReturnedQuantity(tx, repairID, partID)
}

func sumReturnedQuantity(statement *gorm.DB, repairID uint, partID uint) (int, error) {
	var total int
	err := statement.Model(&StockTx{}).
		Where("repair_id = ? AND part_id = ? AND type = ?", repairID, partID, TxReturn).
		Select("COALESCE(SUM(quantity), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("统计退料数量失败: %w", err)
	}
	return total, nil
}

// ConsumptionRanking 返回备件消耗排名(按维修领用量汇总), 同时取消的冲销不计入。
func (r *Repository) ConsumptionRanking(ctx context.Context, limit int) ([]ConsumptionRank, error) {
	rows := make([]ConsumptionRank, 0)
	query := r.session(ctx).Model(&RepairMaterial{}).
		Select("part_id, part_code, part_name, unit, SUM(quantity) AS total_qty, " +
			"COUNT(DISTINCT repair_id) AS repair_count, SUM(quantity * unit_price) AS total_amount").
		Group("part_id, part_code, part_name, unit").
		Order("total_qty DESC, part_code ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("统计备件消耗排名失败: %w", err)
	}
	return rows, nil
}

// applyPartFilter 统一拼装备件查询条件。
func applyPartFilter(statement *gorm.DB, filter Filter) *gorm.DB {
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		statement = statement.Where(
			"code LIKE ? OR name LIKE ? OR specification LIKE ? OR supplier LIKE ? OR location LIKE ?",
			like, like, like, like, like,
		)
	}
	if value := strings.TrimSpace(filter.Category); value != "" {
		statement = statement.Where("category = ?", value)
	}
	switch strings.TrimSpace(filter.Shortage) {
	case "shortage":
		statement = statement.Where("stock <= safety_stock")
	case "out":
		statement = statement.Where("stock = 0")
	}
	return statement
}

// applyTxFilter 统一拼装库存流水查询条件。
func applyTxFilter(statement *gorm.DB, filter TxFilter) *gorm.DB {
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		statement = statement.Where(
			"tx_no LIKE ? OR part_code LIKE ? OR part_name LIKE ? OR repair_no LIKE ? OR fault_no LIKE ? OR operator LIKE ?",
			like, like, like, like, like, like,
		)
	}
	if filter.PartID > 0 {
		statement = statement.Where("part_id = ?", filter.PartID)
	}
	if value := strings.TrimSpace(filter.Type); value != "" {
		statement = statement.Where("type = ?", value)
	}
	if value := strings.TrimSpace(filter.Operator); value != "" {
		statement = statement.Where("operator = ?", value)
	}
	if filter.OccurredFrom != nil {
		statement = statement.Where("occurred_at >= ?", *filter.OccurredFrom)
	}
	if filter.OccurredTo != nil {
		statement = statement.Where("occurred_at < ?", *filter.OccurredTo)
	}
	return statement
}
