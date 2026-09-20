package bootstrap

import (
	"gorm.io/gorm"

	"streetlight/internal/module"
	"streetlight/internal/modules/fault"
	"streetlight/internal/modules/lamp"
	"streetlight/internal/modules/parts"
	"streetlight/internal/modules/repair"
	"streetlight/internal/modules/status"
)

// buildModules 按依赖方向装配业务模块。
//
// 依赖关系: 路灯台账 <- 故障登记 <- 维修记录, 维修开工/删除联动备件库存,
// 维修状态查询依赖路灯/故障/维修/备件四个模块的只读仓储。
// 其中 "删除路灯前校验未闭环故障" 需要路灯模块反向调用故障模块,
// 因此通过 SetOpenFaultCounter 在构造完成后回填, 避免构造函数循环依赖。
func buildModules(db *gorm.DB) []module.Module {
	lampModule := lamp.New(db)

	faultModule := fault.New(db, lampModule.Service())
	lampModule.Service().SetOpenFaultCounter(faultModule.Repository())

	partsModule := parts.New(db)

	repairModule := repair.New(db, faultModule.Service(), partsModule.Service())

	statusModule := status.New(
		db,
		lampModule.Repository(),
		faultModule.Repository(),
		repairModule.Repository(),
		partsModule.Repository(),
	)

	return []module.Module{
		lampModule,
		faultModule,
		partsModule,
		repairModule,
		statusModule,
	}
}
