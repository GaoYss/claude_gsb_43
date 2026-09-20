package parts

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Module 备件与耗材库存模块, 负责备件台账、库存流水与维修用料明细。
type Module struct {
	repository *Repository
	service    *Service
	handler    *Handler
}

// New 构造备件库存模块。
func New(db *gorm.DB) *Module {
	repository := NewRepository(db)
	service := NewService(repository)
	return &Module{
		repository: repository,
		service:    service,
		handler:    NewHandler(service),
	}
}

// Service 暴露模块业务服务, 供维修模块装配领用/冲销端口。
func (m *Module) Service() *Service { return m.service }

// Repository 暴露仓储, 供状态查询模块装配只读视图。
func (m *Module) Repository() *Repository { return m.repository }

// Name 实现 module.Module 接口。
func (m *Module) Name() string { return "备件库存" }

// Models 实现 module.Module 接口。
func (m *Module) Models() []any {
	return []any{&Part{}, &StockTx{}, &RepairMaterial{}}
}

// RegisterRoutes 实现 module.Module 接口。
func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
	group := api.Group("/parts")
	{
		group.GET("", m.handler.ListParts)
		group.POST("", m.handler.CreatePart)
		group.GET("/options", m.handler.Options)
		group.GET("/statistics", m.handler.Statistics)
		group.GET("/shortage", m.handler.ShortageList)
		group.GET("/consumption-ranking", m.handler.ConsumptionRanking)
		group.GET("/transactions", m.handler.ListStockTx)
		group.POST("/transactions", m.handler.CreateStockTx)
		group.GET("/materials/repair/:repairId", m.handler.ListMaterials)
		group.GET("/:id", m.handler.GetPart)
		group.PUT("/:id", m.handler.UpdatePart)
		group.DELETE("/:id", m.handler.DeletePart)
	}
}
