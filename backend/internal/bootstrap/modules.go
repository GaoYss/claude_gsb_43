package bootstrap

import (
	"gorm.io/gorm"

	"streetlight/internal/module"
	"streetlight/internal/modules/fault"
	"streetlight/internal/modules/inventory"
	"streetlight/internal/modules/lamp"
	"streetlight/internal/modules/repair"
	"streetlight/internal/modules/status"
)

// buildModules 按依赖方向装配业务模块。
//
// 依赖关系: 路灯台账 <- 故障登记 <- 维修记录, 维修开工还要扣减备件库存;
// 维修状态查询依赖各业务模块的只读仓储做聚合。
// 其中 "删除路灯前校验未闭环故障" 需要路灯模块反向调用故障模块,
// 因此通过 SetOpenFaultCounter 在构造完成后回填, 避免循环构造依赖。
func buildModules(db *gorm.DB) []module.Module {
	lampModule := lamp.New(db)

	faultModule := fault.New(db, lampModule.Service())
	lampModule.Service().SetOpenFaultCounter(faultModule.Repository())

	// 备件与耗材库存独立成账, 维修开工领用自动扣减。
	inventoryModule := inventory.New(db)

	repairModule := repair.New(db, faultModule.Service(), inventoryModule.Service())

	statusModule := status.New(
		db,
		lampModule.Repository(),
		faultModule.Repository(),
		repairModule.Repository(),
		inventoryModule.Repository(),
	)

	return []module.Module{
		lampModule,
		faultModule,
		inventoryModule,
		repairModule,
		statusModule,
	}
}
