package inventory

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"streetlight/internal/apperr"
	"streetlight/pkg/pagination"
)

// Filter 是仓储层使用的备件台账查询条件。
type PartFilter struct {
	Keyword  string
	Category string
	Status   string
	Shortage bool
}

// TxnFilter 是出入库流水查询条件。
type TxnFilter struct {
	Keyword  string
	TxType   string
	PartID   uint
	RepairNo string
	FaultNo  string
	Operator string
	From     *time.Time
	To       *time.Time
}

// Repository 负责备件、出入库流水与维修用料明细的数据访问。
type Repository struct {
	db *gorm.DB
}

// NewRepository 构造备件库存仓储。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) session(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

// Transaction 在一个数据库事务中执行 fn, fn 内使用的仓储与外部共享同一事务连接。
func (r *Repository) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

// ---- 备件台账 ----

// CreatePart 新增备件。
func (r *Repository) CreatePart(ctx context.Context, part *SparePart) error {
	if err := r.session(ctx).Create(part).Error; err != nil {
		return fmt.Errorf("新增备件失败: %w", err)
	}
	return nil
}

// UpdatePart 保存备件档案变更。
func (r *Repository) UpdatePart(ctx context.Context, part *SparePart) error {
	if err := r.session(ctx).Save(part).Error; err != nil {
		return fmt.Errorf("更新备件失败: %w", err)
	}
	return nil
}

// UpdateColumns 按列更新备件。
func (r *Repository) UpdateColumns(ctx context.Context, id uint, columns map[string]any) error {
	if err := r.session(ctx).Model(&SparePart{}).Where("id = ?", id).Updates(columns).Error; err != nil {
		return fmt.Errorf("更新备件失败: %w", err)
	}
	return nil
}

// DeletePart 按主键删除备件。
func (r *Repository) DeletePart(ctx context.Context, id uint) error {
	if err := r.session(ctx).Delete(&SparePart{}, id).Error; err != nil {
		return fmt.Errorf("删除备件失败: %w", err)
	}
	return nil
}

// GetPartByID 按主键查询备件。
func (r *Repository) GetPartByID(ctx context.Context, id uint) (*SparePart, error) {
	var part SparePart
	err := r.session(ctx).First(&part, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("备件不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询备件失败: %w", err)
	}
	return &part, nil
}

// GetPartByIDForUpdate 在事务内按主键查询备件并加行锁, 保证库存扣减的并发安全。
func (r *Repository) GetPartByIDForUpdate(tx *gorm.DB, id uint) (*SparePart, error) {
	var part SparePart
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&part, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("备件不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询备件失败: %w", err)
	}
	return &part, nil
}

// GetPartByCode 按备件编号查询, 不存在时返回 nil。
func (r *Repository) GetPartByCode(ctx context.Context, code string) (*SparePart, error) {
	var part SparePart
	err := r.session(ctx).Where("code = ?", code).First(&part).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询备件失败: %w", err)
	}
	return &part, nil
}

// ListParts 分页查询备件台账。
func (r *Repository) ListParts(ctx context.Context, filter PartFilter, page pagination.Query) ([]SparePart, int64, error) {
	base := func() *gorm.DB {
		return applyPartFilter(r.session(ctx).Model(&SparePart{}), filter)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计备件失败: %w", err)
	}

	items := make([]SparePart, 0)
	if err := base().Order(page.OrderClause()).Offset(page.Offset()).Limit(page.Limit()).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("查询备件失败: %w", err)
	}
	return items, total, nil
}

// ListPartsByIDs 按主键批量查询备件。
func (r *Repository) ListPartsByIDs(ctx context.Context, ids []uint) ([]SparePart, error) {
	items := make([]SparePart, 0)
	if len(ids) == 0 {
		return items, nil
	}
	if err := r.session(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("批量查询备件失败: %w", err)
	}
	return items, nil
}

// ListShortage 查询缺货(在用且库存不高于安全库存)的备件。
func (r *Repository) ListShortage(ctx context.Context, limit int) ([]SparePart, error) {
	items := make([]SparePart, 0)
	statement := r.session(ctx).
		Where("status = ? AND stock <= safety_stock", PartStatusActive).
		Order("stock ASC, id ASC")
	if limit > 0 {
		statement = statement.Limit(limit)
	}
	if err := statement.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("查询缺货备件失败: %w", err)
	}
	return items, nil
}

// CountShortage 统计缺货备件数量。
func (r *Repository) CountShortage(ctx context.Context) (int64, error) {
	var total int64
	err := r.session(ctx).Model(&SparePart{}).
		Where("status = ? AND stock <= safety_stock", PartStatusActive).
		Count(&total).Error
	if err != nil {
		return 0, fmt.Errorf("统计缺货备件失败: %w", err)
	}
	return total, nil
}

// CountParts 统计备件总数与按状态分组数量。
func (r *Repository) CountParts(ctx context.Context) (int64, map[string]int64, error) {
	var total int64
	if err := r.session(ctx).Model(&SparePart{}).Count(&total).Error; err != nil {
		return 0, nil, fmt.Errorf("统计备件总数失败: %w", err)
	}
	type row struct {
		Label string
		Total int64
	}
	rows := make([]row, 0)
	if err := r.session(ctx).Model(&SparePart{}).
		Select("status AS label, COUNT(*) AS total").
		Group("status").Scan(&rows).Error; err != nil {
		return 0, nil, fmt.Errorf("按状态统计备件失败: %w", err)
	}
	byStatus := make(map[string]int64, len(rows))
	for _, item := range rows {
		byStatus[item.Label] = item.Total
	}
	return total, byStatus, nil
}

// StockTotals 汇总在库备件总数量与库存金额。
func (r *Repository) StockTotals(ctx context.Context) (int64, float64, error) {
	type row struct {
		Qty   int64
		Value float64
	}
	var result row
	err := r.session(ctx).Model(&SparePart{}).
		Select("COALESCE(SUM(stock), 0) AS qty, COALESCE(SUM(stock * unit_price), 0) AS value").
		Scan(&result).Error
	if err != nil {
		return 0, 0, fmt.Errorf("汇总库存价值失败: %w", err)
	}
	return result.Qty, result.Value, nil
}

// DistinctPartValues 返回备件某列的去重取值。
func (r *Repository) DistinctPartValues(ctx context.Context, column string) ([]string, error) {
	values := make([]string, 0)
	err := r.session(ctx).Model(&SparePart{}).
		Where(column+" <> ''").Distinct().Order(column).Pluck(column, &values).Error
	if err != nil {
		return nil, fmt.Errorf("查询备件 %s 选项失败: %w", column, err)
	}
	return values, nil
}

// CountPartTransactions 统计备件关联的流水数量, 用于删除前校验。
func (r *Repository) CountPartTransactions(ctx context.Context, partID uint) (int64, error) {
	var total int64
	if err := r.session(ctx).Model(&StockTransaction{}).Where("part_id = ?", partID).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("统计备件流水失败: %w", err)
	}
	return total, nil
}

// NextPartCode 依据已有编号给出建议的下一个备件编号 BJ + 4 位序号。
func (r *Repository) NextPartCode(ctx context.Context) (string, error) {
	var latest string
	err := r.session(ctx).Model(&SparePart{}).
		Where("code LIKE ?", "BJ%").Order("code DESC").Limit(1).Pluck("code", &latest).Error
	if err != nil {
		return "", fmt.Errorf("生成备件编号失败: %w", err)
	}
	if latest == "" {
		return "BJ0001", nil
	}
	value, convErr := strconv.Atoi(strings.TrimPrefix(latest, "BJ"))
	if convErr != nil {
		return "BJ0001", nil
	}
	return fmt.Sprintf("BJ%04d", value+1), nil
}

// ---- 出入库流水 ----

// CreateTransaction 落库一条出入库流水。
func (r *Repository) CreateTransaction(ctx context.Context, txn *StockTransaction) error {
	if err := r.session(ctx).Create(txn).Error; err != nil {
		return fmt.Errorf("写入出入库流水失败: %w", err)
	}
	return nil
}

// CreateTransactionWithUniqueNo 生成唯一流水单号并落库, 冲突时自动重试。
func (r *Repository) CreateTransactionWithUniqueNo(tx *gorm.DB, txn *StockTransaction, prefix string) error {
	for attempt := 0; attempt < 5; attempt++ {
		sequence, err := r.nextTxnSequence(tx, prefix)
		if err != nil {
			return err
		}
		txn.TxNo = fmt.Sprintf("%s%04d", prefix, sequence+attempt)
		err = tx.Create(txn).Error
		if err == nil {
			return nil
		}
		if !isUniqueViolation(err) {
			return fmt.Errorf("写入出入库流水失败: %w", err)
		}
	}
	return apperr.Conflict("出入库流水单号生成冲突, 请稍后重试")
}

func (r *Repository) nextTxnSequence(tx *gorm.DB, prefix string) (int, error) {
	var latest string
	err := tx.Model(&StockTransaction{}).
		Where("tx_no LIKE ?", prefix+"%").
		Order("tx_no DESC").Limit(1).Pluck("tx_no", &latest).Error
	if err != nil {
		return 0, fmt.Errorf("生成流水单号失败: %w", err)
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

// ListTransactions 分页查询出入库流水。
func (r *Repository) ListTransactions(ctx context.Context, filter TxnFilter, page pagination.Query) ([]StockTransaction, int64, error) {
	base := func() *gorm.DB {
		return applyTxnFilter(r.session(ctx).Model(&StockTransaction{}), filter)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计出入库流水失败: %w", err)
	}

	items := make([]StockTransaction, 0)
	if err := base().Order(page.OrderClause()).Offset(page.Offset()).Limit(page.Limit()).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("查询出入库流水失败: %w", err)
	}
	return items, total, nil
}

// ---- 维修用料明细 ----

// CreateMaterials 批量写入维修用料明细。
func (r *Repository) CreateMaterials(tx *gorm.DB, materials []RepairMaterial) error {
	if len(materials) == 0 {
		return nil
	}
	if err := tx.Create(&materials).Error; err != nil {
		return fmt.Errorf("写入维修用料明细失败: %w", err)
	}
	return nil
}

// UpdateMaterial 保存用料明细变更(退料/报废数量)。
func (r *Repository) UpdateMaterial(ctx context.Context, material *RepairMaterial) error {
	if err := r.session(ctx).Save(material).Error; err != nil {
		return fmt.Errorf("更新维修用料明细失败: %w", err)
	}
	return nil
}

// GetMaterialByID 按主键查询用料明细。
func (r *Repository) GetMaterialByID(ctx context.Context, id uint) (*RepairMaterial, error) {
	var material RepairMaterial
	err := r.session(ctx).First(&material, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("维修用料明细不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询维修用料明细失败: %w", err)
	}
	return &material, nil
}

// GetMaterialByIDForUpdate 在事务内按主键查询用料明细并加行锁。
func (r *Repository) GetMaterialByIDForUpdate(tx *gorm.DB, id uint) (*RepairMaterial, error) {
	var material RepairMaterial
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&material, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("维修用料明细不存在: id=%d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("查询维修用料明细失败: %w", err)
	}
	return &material, nil
}

// ListMaterialsByRepair 查询某次维修的全部用料明细。
func (r *Repository) ListMaterialsByRepair(ctx context.Context, repairID uint) ([]RepairMaterial, error) {
	items := make([]RepairMaterial, 0)
	err := r.session(ctx).Where("repair_id = ?", repairID).Order("id ASC").Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("查询维修用料明细失败: %w", err)
	}
	return items, nil
}

// ListMaterialsByRepairs 批量查询多次维修的用料明细。
func (r *Repository) ListMaterialsByRepairs(ctx context.Context, repairIDs []uint) (map[uint][]RepairMaterial, error) {
	result := make(map[uint][]RepairMaterial)
	if len(repairIDs) == 0 {
		return result, nil
	}
	items := make([]RepairMaterial, 0)
	if err := r.session(ctx).Where("repair_id IN ?", repairIDs).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("批量查询维修用料明细失败: %w", err)
	}
	for index := range items {
		item := items[index]
		result[item.RepairID] = append(result[item.RepairID], item)
	}
	return result, nil
}

// DeleteMaterialsByRepair 删除某次维修的全部用料明细(删除维修记录时联动)。
func (r *Repository) DeleteMaterialsByRepair(tx *gorm.DB, repairID uint) error {
	if err := tx.Where("repair_id = ?", repairID).Delete(&RepairMaterial{}).Error; err != nil {
		return fmt.Errorf("删除维修用料明细失败: %w", err)
	}
	return nil
}

// ConsumptionRanking 按净消耗量倒序统计备件消耗排名。
func (r *Repository) ConsumptionRanking(ctx context.Context, limit int) ([]ConsumptionRow, error) {
	type row struct {
		PartID      uint
		PartCode    string
		PartName    string
		Spec        string
		Unit        string
		IssuedQty   int
		ReturnedQty int
		ConsumedQty int
		RepairCount int64
		Value       float64
	}
	rows := make([]row, 0)
	statement := r.session(ctx).Model(&RepairMaterial{}).
		Select(`part_id, part_code, part_name, spec, unit,
			SUM(quantity) AS issued_qty,
			COALESCE(SUM(returned_qty), 0) AS returned_qty,
			SUM(quantity - returned_qty) AS consumed_qty,
			COUNT(DISTINCT repair_id) AS repair_count,
			SUM((quantity - returned_qty) * unit_price) AS value`).
		Group("part_id").
		Order("consumed_qty DESC, part_code ASC")
	if limit > 0 {
		statement = statement.Limit(limit)
	}
	if err := statement.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("统计备件消耗排名失败: %w", err)
	}
	result := make([]ConsumptionRow, 0, len(rows))
	for _, item := range rows {
		result = append(result, ConsumptionRow{
			PartID:        item.PartID,
			PartCode:      item.PartCode,
			PartName:      item.PartName,
			Spec:          item.Spec,
			Unit:          item.Unit,
			IssuedQty:     item.IssuedQty,
			ReturnedQty:   item.ReturnedQty,
			ConsumedQty:   item.ConsumedQty,
			RepairCount:   item.RepairCount,
			ConsumedValue: item.Value,
		})
	}
	return result, nil
}

// ---- 查询条件拼装 ----

func applyPartFilter(statement *gorm.DB, filter PartFilter) *gorm.DB {
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		statement = statement.Where("code LIKE ? OR name LIKE ? OR spec LIKE ? OR supplier LIKE ?", like, like, like, like)
	}
	if value := strings.TrimSpace(filter.Category); value != "" {
		statement = statement.Where("category = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		statement = statement.Where("status = ?", value)
	}
	if filter.Shortage {
		statement = statement.Where("status = ? AND stock <= safety_stock", PartStatusActive)
	}
	return statement
}

func applyTxnFilter(statement *gorm.DB, filter TxnFilter) *gorm.DB {
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		statement = statement.Where("tx_no LIKE ? OR part_code LIKE ? OR part_name LIKE ?", like, like, like)
	}
	if filter.TxType != "" {
		statement = statement.Where("tx_type = ?", filter.TxType)
	}
	if filter.PartID > 0 {
		statement = statement.Where("part_id = ?", filter.PartID)
	}
	if value := strings.TrimSpace(filter.RepairNo); value != "" {
		statement = statement.Where("repair_no = ?", value)
	}
	if value := strings.TrimSpace(filter.FaultNo); value != "" {
		statement = statement.Where("fault_no = ?", value)
	}
	if value := strings.TrimSpace(filter.Operator); value != "" {
		statement = statement.Where("operator = ?", value)
	}
	if filter.From != nil {
		statement = statement.Where("occurred_at >= ?", *filter.From)
	}
	if filter.To != nil {
		statement = statement.Where("occurred_at < ?", *filter.To)
	}
	return statement
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
