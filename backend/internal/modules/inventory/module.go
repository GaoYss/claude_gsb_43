package inventory

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Module 备件与耗材库存模块, 负责备件台账、出入库流水与维修用料联动。
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

// Service 暴露库存服务, 供维修模块装配领用/退料/回退端口。
func (m *Module) Service() *Service { return m.service }

// Repository 暴露仓储, 供状态查询模块装配只读视图。
func (m *Module) Repository() *Repository { return m.repository }

// Name 实现 module.Module 接口。
func (m *Module) Name() string { return "备件库存" }

// Models 实现 module.Module 接口。
func (m *Module) Models() []any {
	return []any{&SparePart{}, &StockTransaction{}, &RepairMaterial{}}
}

// RegisterRoutes 实现 module.Module 接口。
func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
	parts := api.Group("/inventory/parts")
	{
		parts.GET("", m.handler.ListParts)
		parts.POST("", m.handler.CreatePart)
		parts.GET("/options", m.handler.Options)
		parts.GET("/meta", m.handler.Metadata)
		parts.GET("/statistics", m.handler.Statistics)
		parts.GET("/consumption", m.handler.Consumption)
		parts.GET("/shortage", m.handler.Shortage)
		parts.GET("/:id", m.handler.GetPart)
		parts.PUT("/:id", m.handler.UpdatePart)
		parts.DELETE("/:id", m.handler.DeletePart)
		parts.POST("/:id/inbound", m.handler.Inbound)
		parts.POST("/:id/scrap", m.handler.ScrapPart)
	}

	materials := api.Group("/inventory/materials")
	{
		materials.GET("/repair/:repairId", m.handler.ListMaterials)
		materials.POST("/:id/return", m.handler.ReturnMaterial)
		materials.POST("/:id/scrap", m.handler.ScrapMaterial)
	}

	txns := api.Group("/inventory/transactions")
	{
		txns.GET("", m.handler.ListTransactions)
	}
}
