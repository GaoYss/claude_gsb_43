package repair_test

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
	"streetlight/internal/modules/fault"
	"streetlight/internal/modules/lamp"
	"streetlight/internal/modules/parts"
	"streetlight/internal/modules/repair"
)

// harness 使用内存数据库装配真实模块, 用于验证跨模块业务流程。
type harness struct {
	lamps   *lamp.Service
	faults  *fault.Service
	repairs *repair.Service
	parts   *parts.Service
	db      *gorm.DB
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(&lamp.Lamp{}, &fault.Fault{}, &repair.Repair{},
		&parts.Part{}, &parts.StockTx{}, &parts.RepairMaterial{}))

	lampRepository := lamp.NewRepository(db)
	lampService := lamp.NewService(lampRepository)

	faultRepository := fault.NewRepository(db)
	faultService := fault.NewService(faultRepository, lampService)
	lampService.SetOpenFaultCounter(faultRepository)

	partsRepository := parts.NewRepository(db)
	partsService := parts.NewService(partsRepository)

	repairRepository := repair.NewRepository(db)
	repairService := repair.NewService(repairRepository, faultService, partsService)

	return &harness{
		lamps:   lampService,
		faults:  faultService,
		repairs: repairService,
		parts:   partsService,
		db:      db,
	}
}

func (h *harness) createPart(t *testing.T, code, name string, stock, safety int) *parts.Part {
	t.Helper()
	entity, err := h.parts.CreatePart(context.Background(), parts.PartCreateRequest{
		Code: code, Name: name, Unit: "个", Stock: &stock, SafetyStock: &safety,
	})
	require.NoError(t, err)
	return entity
}

func (h *harness) createLamp(t *testing.T, code string) *lamp.Lamp {
	t.Helper()
	entity, err := h.lamps.Create(context.Background(), lamp.CreateRequest{
		Code:     code,
		Name:     "测试灯杆",
		RoadName: "测试路",
		LampType: lamp.LampTypeLED,
	})
	require.NoError(t, err)
	return entity
}

func (h *harness) createFault(t *testing.T, lampID uint, description string) *fault.Fault {
	t.Helper()
	entity, err := h.faults.Create(context.Background(), fault.CreateRequest{
		LampID:      lampID,
		FaultType:   "灯不亮",
		FaultLevel:  fault.LevelHigh,
		Source:      fault.SourceInspection,
		Description: description,
		Reporter:    "巡检员",
	})
	require.NoError(t, err)
	return entity
}

// requireConflict 断言错误是 409 业务冲突。
func requireConflict(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	businessErr, ok := apperr.As(err)
	require.True(t, ok, "期望业务错误, 实际: %v", err)
	require.Equal(t, http.StatusConflict, businessErr.Status, "错误信息: %s", businessErr.Message)
}

func TestFaultRepairLifecycle(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	device := h.createLamp(t, "LD-T-001")

	entity := h.createFault(t, device.ID, "整灯不亮, 疑似驱动电源故障")
	require.Equal(t, fault.StatusPending, entity.Status)
	require.Regexp(t, `^GD\d{8}\d{4}$`, entity.FaultNo)

	afterReport, err := h.lamps.Get(ctx, device.ID)
	require.NoError(t, err)
	require.Equal(t, lamp.RunStatusFault, afterReport.RunStatus, "登记故障后路灯应变为故障状态")

	// 同一盏路灯不允许存在多条未闭环故障
	_, err = h.faults.Create(ctx, fault.CreateRequest{
		LampID: device.ID, FaultType: "灯不亮", Description: "重复登记",
	})
	requireConflict(t, err)

	// 维修开工
	record, err := h.repairs.Create(ctx, repair.CreateRequest{
		FaultID: entity.ID, Repairman: "维修工甲", RepairTeam: "市政照明一班",
	})
	require.NoError(t, err)
	require.Equal(t, repair.StatusOngoing, record.Status)
	require.Regexp(t, `^WX\d{8}\d{4}$`, record.RepairNo)

	faultAfterStart, err := h.faults.GetByID(ctx, entity.ID)
	require.NoError(t, err)
	require.Equal(t, fault.StatusProcessing, faultAfterStart.Status)
	require.Equal(t, 1, faultAfterStart.RepairCount)
	require.NotNil(t, faultAfterStart.LatestRepairID)
	require.Equal(t, record.ID, *faultAfterStart.LatestRepairID)

	lampAfterStart, err := h.lamps.Get(ctx, device.ID)
	require.NoError(t, err)
	require.Equal(t, lamp.RunStatusMaintenance, lampAfterStart.RunStatus)

	// 同一故障不允许并行开工
	_, err = h.repairs.Create(ctx, repair.CreateRequest{FaultID: entity.ID, Repairman: "维修工乙"})
	requireConflict(t, err)

	// 完工且结果为已修复
	cost := 210.0
	finished, err := h.repairs.Finish(ctx, record.ID, repair.FinishRequest{
		Result: repair.ResultFixed, Content: "更换驱动电源", Cost: &cost,
	})
	require.NoError(t, err)
	require.Equal(t, repair.StatusFinished, finished.Status)
	require.NotNil(t, finished.FinishedAt)

	faultAfterFinish, err := h.faults.GetByID(ctx, entity.ID)
	require.NoError(t, err)
	require.Equal(t, fault.StatusRepaired, faultAfterFinish.Status)

	lampAfterFinish, err := h.lamps.Get(ctx, device.ID)
	require.NoError(t, err)
	require.Equal(t, lamp.RunStatusNormal, lampAfterFinish.RunStatus, "修复后路灯应恢复为正常")

	// 关闭故障形成闭环
	closed, err := h.faults.Close(ctx, entity.ID, fault.CloseRequest{Remark: "现场复核通过"})
	require.NoError(t, err)
	require.Equal(t, fault.StatusClosed, closed.Status)
	require.NotNil(t, closed.ClosedAt)

	// 已产生的维修记录使故障不可删除
	requireConflict(t, h.faults.Delete(ctx, entity.ID))
	// 已关闭故障不允许再次关闭
	_, err = h.faults.Close(ctx, entity.ID, fault.CloseRequest{})
	requireConflict(t, err)
}

func TestRepairPendingPartsKeepsFaultProcessing(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	device := h.createLamp(t, "LD-T-002")
	entity := h.createFault(t, device.ID, "线路老化需要更换电缆")

	record, err := h.repairs.Create(ctx, repair.CreateRequest{
		FaultID: entity.ID, Repairman: "维修工丙",
	})
	require.NoError(t, err)

	_, err = h.repairs.Finish(ctx, record.ID, repair.FinishRequest{Result: repair.ResultPendingParts})
	require.NoError(t, err)

	faultAfterFinish, err := h.faults.GetByID(ctx, entity.ID)
	require.NoError(t, err)
	require.Equal(t, fault.StatusProcessing, faultAfterFinish.Status, "非已修复结果不应结束故障")

	lampAfterFinish, err := h.lamps.Get(ctx, device.ID)
	require.NoError(t, err)
	require.Equal(t, lamp.RunStatusMaintenance, lampAfterFinish.RunStatus)

	// 可继续登记第二次维修(返修)
	second, err := h.repairs.Create(ctx, repair.CreateRequest{
		FaultID: entity.ID, Repairman: "维修工丙", Content: "物料到场后更换电缆",
	})
	require.NoError(t, err)
	require.Equal(t, repair.StatusOngoing, second.Status)

	faultAfterSecond, err := h.faults.GetByID(ctx, entity.ID)
	require.NoError(t, err)
	require.Equal(t, 2, faultAfterSecond.RepairCount)
}

func TestRepairRejectedOnClosedFault(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	device := h.createLamp(t, "LD-T-003")
	entity := h.createFault(t, device.ID, "误报故障需要作废")

	_, err := h.faults.Close(ctx, entity.ID, fault.CloseRequest{Remark: "误报作废"})
	require.NoError(t, err)

	_, err = h.repairs.Create(ctx, repair.CreateRequest{FaultID: entity.ID, Repairman: "维修工丁"})
	requireConflict(t, err)
}

func TestFaultValidation(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	device := h.createLamp(t, "LD-T-004")

	// 非法故障类型
	_, err := h.faults.Create(ctx, fault.CreateRequest{
		LampID: device.ID, FaultType: "不存在的类型", Description: "测试",
	})
	businessErr, ok := apperr.As(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, businessErr.Status)

	// 路灯不存在
	_, err = h.faults.Create(ctx, fault.CreateRequest{
		LampID: 99999, FaultType: "灯不亮", Description: "测试",
	})
	businessErr, ok = apperr.As(err)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, businessErr.Status)

	// 重复路灯编号
	_, err = h.lamps.Create(ctx, lamp.CreateRequest{
		Code: device.Code, RoadName: "测试路", LampType: lamp.LampTypeLED,
	})
	requireConflict(t, err)

	// 存在未闭环故障时不允许删除路灯
	h.createFault(t, device.ID, "删除校验")
	requireConflict(t, h.lamps.Delete(ctx, device.ID))
}

func TestRepairIssuePartsBlocksOnShortageAndRestoresOnDelete(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	device := h.createLamp(t, "LD-T-005")
	entity := h.createFault(t, device.ID, "更换驱动电源")

	driver := h.createPart(t, "BJ-T-001", "LED 驱动电源", 2, 1)
	fuse := h.createPart(t, "BJ-T-002", "熔断器", 10, 2)

	// 库存不足时拦住开工, 错误信息带可用数量
	_, err := h.repairs.Create(ctx, repair.CreateRequest{
		FaultID: entity.ID, Repairman: "维修工甲",
		MaterialItems: []repair.RepairMaterialRequest{
			{PartID: driver.ID, Quantity: 3},
		},
	})
	requireConflict(t, err)
	require.Contains(t, err.Error(), "可用 2")

	// 备件未被动用
	partAfterBlock, err := h.parts.GetPart(ctx, driver.ID)
	require.NoError(t, err)
	require.Equal(t, 2, partAfterBlock.Stock)

	// 库存充足时开工, 自动扣减库存并保留用料明细
	record, err := h.repairs.Create(ctx, repair.CreateRequest{
		FaultID: entity.ID, Repairman: "维修工甲",
		MaterialItems: []repair.RepairMaterialRequest{
			{PartID: driver.ID, Quantity: 2},
			{PartID: fuse.ID, Quantity: 1},
		},
	})
	require.NoError(t, err)

	driverAfterIssue, err := h.parts.GetPart(ctx, driver.ID)
	require.NoError(t, err)
	require.Equal(t, 0, driverAfterIssue.Stock)
	fuseAfterIssue, err := h.parts.GetPart(ctx, fuse.ID)
	require.NoError(t, err)
	require.Equal(t, 9, fuseAfterIssue.Stock)

	materials, err := h.parts.ListMaterials(ctx, record.ID)
	require.NoError(t, err)
	require.Len(t, materials, 2)

	detail, err := h.repairs.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Len(t, detail.MaterialItems, 2)

	// 删除维修记录自动冲销回补
	require.NoError(t, h.repairs.Delete(ctx, record.ID))
	driverAfterRestore, err := h.parts.GetPart(ctx, driver.ID)
	require.NoError(t, err)
	require.Equal(t, 2, driverAfterRestore.Stock)
	fuseAfterRestore, err := h.parts.GetPart(ctx, fuse.ID)
	require.NoError(t, err)
	require.Equal(t, 10, fuseAfterRestore.Stock)
}

func TestPartReturnRequiresReasonAndRespectsIssuedQuantity(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	device := h.createLamp(t, "LD-T-006")
	entity := h.createFault(t, device.ID, "灯不亮")
	part := h.createPart(t, "BJ-T-003", "通讯模块", 5, 1)

	record, err := h.repairs.Create(ctx, repair.CreateRequest{
		FaultID: entity.ID, Repairman: "维修工乙",
		MaterialItems: []repair.RepairMaterialRequest{{PartID: part.ID, Quantity: 2}},
	})
	require.NoError(t, err)
	require.Equal(t, 3, mustPart(t, h, part.ID).Stock)

	// 退料必须登记原因
	_, err = h.parts.CreateStockTx(ctx, parts.StockTxCreateRequest{
		PartID: part.ID, Type: parts.TxReturn, Quantity: 1, RepairID: record.ID,
	})
	businessErr, ok := apperr.As(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, businessErr.Status)

	// 正常退料回补库存
	returnTx, err := h.parts.CreateStockTx(ctx, parts.StockTxCreateRequest{
		PartID: part.ID, Type: parts.TxReturn, Quantity: 1, RepairID: record.ID,
		Reason: "现场原模块仍可使用, 换新取消", Operator: "维修工乙",
	})
	require.NoError(t, err)
	require.Equal(t, 4, returnTx.StockAfter)

	// 退料数量不能超过领用量(已退 1, 再退 2 被拦截)
	_, err = h.parts.CreateStockTx(ctx, parts.StockTxCreateRequest{
		PartID: part.ID, Type: parts.TxReturn, Quantity: 2, RepairID: record.ID,
		Reason: "超额退料",
	})
	requireConflict(t, err)

	// 报废也必须登记原因, 且不能超过库存
	_, err = h.parts.CreateStockTx(ctx, parts.StockTxCreateRequest{
		PartID: part.ID, Type: parts.TxScrap, Quantity: 1,
	})
	require.Error(t, err)
	_, err = h.parts.CreateStockTx(ctx, parts.StockTxCreateRequest{
		PartID: part.ID, Type: parts.TxScrap, Quantity: 5, Reason: "运输损坏",
	})
	requireConflict(t, err)
}

func mustPart(t *testing.T, h *harness, id uint) *parts.Part {
	t.Helper()
	entity, err := h.parts.GetPart(context.Background(), id)
	require.NoError(t, err)
	return entity
}
