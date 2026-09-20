package inventory_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"streetlight/internal/apperr"
	"streetlight/internal/modules/inventory"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&inventory.SparePart{}, &inventory.StockTransaction{}, &inventory.RepairMaterial{}))
	return db
}

func newService(t *testing.T) (*inventory.Service, *gorm.DB) {
	db := newDB(t)
	return inventory.NewService(inventory.NewRepository(db)), db
}

func createPart(t *testing.T, svc *inventory.Service, code string, stock, safety int) *inventory.SparePart {
	t.Helper()
	part, err := svc.CreatePart(context.Background(), inventory.PartCreateRequest{
		Code: code, Name: "LED 驱动电源", Category: "电气件", Unit: "个",
		InitialStock: intPtr(stock), SafetyStock: intPtr(safety), UnitPrice: floatPtr(120),
	})
	require.NoError(t, err)
	return part
}

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestCreatePartWritesInitialInbound(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	part := createPart(t, svc, "BJ0001", 10, 2)
	require.Equal(t, 10, part.Stock)

	loaded, err := svc.GetPart(ctx, part.ID)
	require.NoError(t, err)
	require.Equal(t, 10, loaded.Stock)

	// 期初库存自动生成一条入库流水
	_, total, _, err := svc.ListTransactions(ctx, inventory.TxnListQuery{PartID: part.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	options, err := svc.Options(ctx)
	require.NoError(t, err)
	require.Equal(t, "BJ0002", options.NextCode)
}

func TestInboundAndScrapPart(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	part := createPart(t, svc, "BJ0001", 5, 1)

	// 入库 5 个, 单价 240 -> 移动加权平均 (5*120 + 5*240)/10 = 180
	txn, err := svc.Inbound(ctx, part.ID, inventory.InboundRequest{Quantity: 5, UnitPrice: floatPtr(240), Operator: "仓管甲"})
	require.NoError(t, err)
	require.Equal(t, inventory.TxInbound, txn.TxType)
	require.Equal(t, 5, txn.StockBefore)
	require.Equal(t, 10, txn.StockAfter)

	loaded, err := svc.GetPart(ctx, part.ID)
	require.NoError(t, err)
	require.Equal(t, 10, loaded.Stock)
	require.InDelta(t, 180, loaded.UnitPrice, 0.01)

	// 报废必须填写原因
	_, err = svc.ScrapPart(ctx, part.ID, inventory.PartScrapRequest{Quantity: 3})
	bizErr, ok := apperr.As(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, bizErr.Status)

	scrapTxn, err := svc.ScrapPart(ctx, part.ID, inventory.PartScrapRequest{Quantity: 3, Reason: "受潮锈蚀", Operator: "仓管甲"})
	require.NoError(t, err)
	require.Equal(t, inventory.TxScrap, scrapTxn.TxType)
	require.Equal(t, -3, scrapTxn.StockDelta)
	require.Equal(t, 7, scrapTxn.StockAfter)

	// 报废数量不能超过库存
	_, err = svc.ScrapPart(ctx, part.ID, inventory.PartScrapRequest{Quantity: 8, Reason: "超量报废"})
	bizErr, ok = apperr.As(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, bizErr.Status)
}

func TestReserveAndReturnAndScrapMaterial(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	part := createPart(t, svc, "BJ0001", 3, 1)
	other := createPart(t, svc, "BJ0002", 10, 0)

	ref := inventory.RepairRef{ID: 100, RepairNo: "WX202609190001", FaultID: 9, FaultNo: "GD202609190001", LampCode: "LD-00001", Operator: "维修工甲"}

	// 库存不足: 需要 5 个但只有 3 个, 返回 409 且携带可用数量
	err := svc.CheckAvailable(ctx, []inventory.MaterialLine{{PartID: part.ID, Quantity: 5}})
	require.Error(t, err)
	shortage, ok := err.(*inventory.ShortageError)
	require.True(t, ok, "期望库存不足冲突, 实际: %v", err)
	require.Len(t, shortage.Details, 1)
	require.Equal(t, 3, shortage.Details[0].Available)
	require.Equal(t, 5, shortage.Details[0].Need)
	bizErr, ok := apperr.As(err)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, bizErr.Status)

	// 库存未因预检发生变化
	loaded, _ := svc.GetPart(ctx, part.ID)
	require.Equal(t, 3, loaded.Stock)

	// 正式领用: 3 个驱动电源 + 2 个其它备件
	materials, err := svc.ReserveMaterials(ctx, []inventory.MaterialLine{
		{PartID: part.ID, Quantity: 3},
		{PartID: other.ID, Quantity: 2},
	}, ref)
	require.NoError(t, err)
	require.Len(t, materials, 2)

	loaded, _ = svc.GetPart(ctx, part.ID)
	require.Equal(t, 0, loaded.Stock)
	require.True(t, loaded.StockShortage(), "库存 0 且安全库存 1, 应触发缺货")

	// 维修用料明细可查
	list, err := svc.ListMaterialsByRepair(ctx, ref.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	driver := list[0]
	require.Equal(t, 3, driver.Quantity)
	require.Equal(t, 3, driver.OpenQty())

	// 退 1 个未使用的驱动电源, 库存回补, 必须登记原因
	_, err = svc.ReturnMaterial(ctx, driver.ID, inventory.MaterialReturnRequest{Quantity: 1, Reason: "现场判断无需更换"})
	require.NoError(t, err)
	loaded, _ = svc.GetPart(ctx, part.ID)
	require.Equal(t, 1, loaded.Stock)

	// 剩余 2 个中现场报废 1 个, 库存不再变动但留痕
	materialAfterReturn, _ := svc.ListMaterialsByRepair(ctx, ref.ID)
	var driverLine *inventory.RepairMaterial
	for i := range materialAfterReturn {
		if materialAfterReturn[i].PartID == part.ID {
			driverLine = &materialAfterReturn[i]
		}
	}
	require.NotNil(t, driverLine)
	require.Equal(t, 2, driverLine.OpenQty())
	scrapTxn, err := svc.ScrapMaterial(ctx, driverLine.ID, inventory.MaterialScrapRequest{Quantity: 1, Reason: "拆卸时外壳碎裂"})
	require.NoError(t, err)
	require.Equal(t, 0, scrapTxn.StockDelta, "现场报废发生在出库之后, 库存增量应为 0")
	loaded, _ = svc.GetPart(ctx, part.ID)
	require.Equal(t, 1, loaded.Stock, "现场报废不应改变库存")

	// 退料/报废不能超过挂账数量
	_, err = svc.ReturnMaterial(ctx, driverLine.ID, inventory.MaterialReturnRequest{Quantity: 5, Reason: "超量退料"})
	require.Error(t, err)
}

func TestRollbackRepairRestoresOpenQuantity(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	part := createPart(t, svc, "BJ0001", 5, 0)

	ref := inventory.RepairRef{ID: 200, RepairNo: "WX202609190002", FaultNo: "GD202609190002", Operator: "维修工乙"}
	_, err := svc.ReserveMaterials(ctx, []inventory.MaterialLine{{PartID: part.ID, Quantity: 5}}, ref)
	require.NoError(t, err)

	materials, err := svc.ListMaterialsByRepair(ctx, ref.ID)
	require.NoError(t, err)
	// 先退 2 个, 再报废 1 个, 挂账剩余 2 个
	_, err = svc.ReturnMaterial(ctx, materials[0].ID, inventory.MaterialReturnRequest{Quantity: 2, Reason: "多领"})
	require.NoError(t, err)
	_, err = svc.ScrapMaterial(ctx, materials[0].ID, inventory.MaterialScrapRequest{Quantity: 1, Reason: "损坏"})
	require.NoError(t, err)
	loaded, _ := svc.GetPart(ctx, part.ID)
	require.Equal(t, 2, loaded.Stock)

	// 删除维修单: 只回退仍挂账的 2 个, 已退库/已报废的不重复处理
	require.NoError(t, svc.RollbackRepair(ctx, ref.ID))
	loaded, _ = svc.GetPart(ctx, part.ID)
	require.Equal(t, 4, loaded.Stock)

	// 用料明细被清理
	rest, err := svc.ListMaterialsByRepair(ctx, ref.ID)
	require.NoError(t, err)
	require.Empty(t, rest)

	// 流水类型完整: 期初入库 + 领用 + 退料 + 报废 + 回退
	_, total, _, err := svc.ListTransactions(ctx, inventory.TxnListQuery{PartID: part.ID})
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
}

func TestConsumptionRankingAndShortage(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	partA := createPart(t, svc, "BJ0001", 20, 5)
	partB := createPart(t, svc, "BJ0002", 20, 10)

	_, err := svc.ReserveMaterials(ctx, []inventory.MaterialLine{
		{PartID: partA.ID, Quantity: 8},
		{PartID: partB.ID, Quantity: 4},
	}, inventory.RepairRef{ID: 1, RepairNo: "WX001", Operator: "甲"})
	require.NoError(t, err)
	_, err = svc.ReserveMaterials(ctx, []inventory.MaterialLine{
		{PartID: partA.ID, Quantity: 2},
	}, inventory.RepairRef{ID: 2, RepairNo: "WX002", Operator: "乙"})
	require.NoError(t, err)
	// A 退 3 个, 净消耗 7 个
	firstList, err := svc.ListMaterialsByRepair(ctx, 1)
	require.NoError(t, err)
	var aLine uint
	for _, m := range firstList {
		if m.PartID == partA.ID {
			aLine = m.ID
		}
	}
	_, err = svc.ReturnMaterial(ctx, aLine, inventory.MaterialReturnRequest{Quantity: 3, Reason: "退料"})
	require.NoError(t, err)

	ranking, err := svc.ConsumptionRanking(ctx, 10)
	require.NoError(t, err)
	require.Len(t, ranking, 2)
	require.Equal(t, partA.ID, ranking[0].PartID, "A 净消耗 7 应排第一")
	require.Equal(t, 7, ranking[0].ConsumedQty)
	require.Equal(t, int64(2), ranking[0].RepairCount)

	// B 库存 16 > 安全库存 10, 不缺货; 不涉及断言 A: A 剩余 10 > 5 也不缺货。
	// 把 A 报废到安全线以下验证缺货清单
	_, err = svc.ScrapPart(ctx, partA.ID, inventory.PartScrapRequest{Quantity: 8, Reason: "批量过期"})
	require.NoError(t, err)
	shortage, err := svc.ShortageList(ctx, 10)
	require.NoError(t, err)
	var codes []string
	for _, item := range shortage {
		codes = append(codes, item.Code)
	}
	require.Contains(t, codes, "BJ0001")

	stats, err := svc.Statistics(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), stats.Total)
	require.GreaterOrEqual(t, stats.ShortageTotal, int64(1))
}

func TestDeletePartGuards(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	// 有流水但零库存: 拒绝硬删除, 建议停用
	part := createPart(t, svc, "BJ0001", 2, 0)
	_, err := svc.ScrapPart(ctx, part.ID, inventory.PartScrapRequest{Quantity: 2, Reason: "清零"})
	require.NoError(t, err)
	err = svc.DeletePart(ctx, part.ID)
	require.Error(t, err)
	bizErr, ok := apperr.As(err)
	require.True(t, ok)
	require.Equal(t, http.StatusConflict, bizErr.Status)

	// 无流水且零库存的备件可以删除
	fresh, err := svc.CreatePart(ctx, inventory.PartCreateRequest{Code: "BJ9999", Name: "全新备件", Unit: "个"})
	require.NoError(t, err)
	require.NoError(t, svc.DeletePart(ctx, fresh.ID))
}
