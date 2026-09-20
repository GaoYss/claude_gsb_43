package repair

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"streetlight/internal/modules/parts"
)

// Module 维修记录模块, 负责维修过程录入与完工闭环。
type Module struct {
	repository *Repository
	service    *Service
	handler    *Handler
}

// New 构造维修记录模块, faults 为故障模块端口, partsPort 为备件库存模块端口。
func New(db *gorm.DB, faults FaultPort, partsPort PartsPort) *Module {
	repository := NewRepository(db)
	service := NewService(repository, faults, partsPort)
	return &Module{
		repository: repository,
		service:    service,
		handler:    NewHandler(service),
	}
}

// 确保 parts.Service 实现维修模块要求的备件端口。
var _ PartsPort = (*parts.Service)(nil)

// Repository 暴露仓储, 供状态查询模块装配。
func (m *Module) Repository() *Repository { return m.repository }

// Name 实现 module.Module 接口。
func (m *Module) Name() string { return "维修记录" }

// Models 实现 module.Module 接口。
func (m *Module) Models() []any { return []any{&Repair{}} }

// RegisterRoutes 实现 module.Module 接口。
func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
	group := api.Group("/repairs")
	{
		group.GET("", m.handler.List)
		group.POST("", m.handler.Create)
		group.GET("/meta", m.handler.Metadata)
		group.GET("/statistics", m.handler.Statistics)
		group.GET("/fault/:faultId", m.handler.ListByFault)
		group.GET("/:id", m.handler.Get)
		group.PUT("/:id", m.handler.Update)
		group.POST("/:id/finish", m.handler.Finish)
		group.DELETE("/:id", m.handler.Delete)
	}
}
