package inventory

import (
	"github.com/gin-gonic/gin"

	"streetlight/internal/httpx"
	"streetlight/internal/response"
)

// Handler 处理备件库存相关的 HTTP 请求。
type Handler struct {
	service *Service
}

// NewHandler 构造备件库存处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListParts 分页查询备件台账。
func (h *Handler) ListParts(c *gin.Context) {
	var query PartListQuery
	if err := httpx.BindQuery(c, &query); err != nil {
		response.Fail(c, err)
		return
	}
	items, total, page, err := h.service.ListParts(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.NewPageData(items, total, page.Page, page.PageSize))
}

// CreatePart 新增备件档案。
func (h *Handler) CreatePart(c *gin.Context) {
	var req PartCreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	part, err := h.service.CreatePart(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, part)
}

// GetPart 查询备件详情。
func (h *Handler) GetPart(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	part, err := h.service.GetPart(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, part)
}

// UpdatePart 修改备件档案。
func (h *Handler) UpdatePart(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req PartUpdateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	part, err := h.service.UpdatePart(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, part)
}

// DeletePart 删除备件。
func (h *Handler) DeletePart(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.service.DeletePart(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// Inbound 备件入库。
func (h *Handler) Inbound(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req InboundRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	txn, err := h.service.Inbound(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, txn)
}

// ScrapPart 库存备件报废。
func (h *Handler) ScrapPart(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req PartScrapRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	txn, err := h.service.ScrapPart(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, txn)
}

// ListTransactions 分页查询出入库流水。
func (h *Handler) ListTransactions(c *gin.Context) {
	var query TxnListQuery
	if err := httpx.BindQuery(c, &query); err != nil {
		response.Fail(c, err)
		return
	}
	items, total, page, err := h.service.ListTransactions(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.NewPageData(items, total, page.Page, page.PageSize))
}

// ListMaterials 查询某次维修的用料明细。
func (h *Handler) ListMaterials(c *gin.Context) {
	repairID, err := httpx.ParseID(c, "repairId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	items, err := h.service.ListMaterialsByRepair(c.Request.Context(), repairID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, items)
}

// ReturnMaterial 维修退料登记。
func (h *Handler) ReturnMaterial(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req MaterialReturnRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	txn, err := h.service.ReturnMaterial(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, txn)
}

// ScrapMaterial 维修现场报废登记。
func (h *Handler) ScrapMaterial(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req MaterialScrapRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	txn, err := h.service.ScrapMaterial(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, txn)
}

// Options 返回备件下拉选项与建议编号。
func (h *Handler) Options(c *gin.Context) {
	options, err := h.service.Options(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, options)
}

// Metadata 返回库存模块字典。
func (h *Handler) Metadata(c *gin.Context) {
	response.OK(c, h.service.Metadata())
}

// Statistics 返回备件库存统计。
func (h *Handler) Statistics(c *gin.Context) {
	statistics, err := h.service.Statistics(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, statistics)
}

// Consumption 备件消耗排名。
func (h *Handler) Consumption(c *gin.Context) {
	var query struct {
		Limit int `form:"limit"`
	}
	if err := httpx.BindQuery(c, &query); err != nil {
		response.Fail(c, err)
		return
	}
	rows, err := h.service.ConsumptionRanking(c.Request.Context(), query.Limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// Shortage 缺货预警清单。
func (h *Handler) Shortage(c *gin.Context) {
	var query struct {
		Limit int `form:"limit"`
	}
	if err := httpx.BindQuery(c, &query); err != nil {
		response.Fail(c, err)
		return
	}
	rows, err := h.service.ShortageList(c.Request.Context(), query.Limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}
