package parts

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"streetlight/internal/apperr"
	"streetlight/internal/httpx"
	"streetlight/internal/response"
)

// Handler 处理备件台账与库存流水相关的 HTTP 请求。
type Handler struct {
	service *Service
}

// NewHandler 构造备件库存处理器。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListParts 查询备件台账列表。
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

// GetPart 查询备件详情。
func (h *Handler) GetPart(c *gin.Context) {
	id, err := httpx.ParseID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	entity, err := h.service.GetPart(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, entity)
}

// CreatePart 新增备件台账。
func (h *Handler) CreatePart(c *gin.Context) {
	var req PartCreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	entity, err := h.service.CreatePart(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, entity)
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
	entity, err := h.service.UpdatePart(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, entity)
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

// Statistics 备件库存统计。
func (h *Handler) Statistics(c *gin.Context) {
	statistics, err := h.service.Statistics(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, statistics)
}

// Options 备件模块下拉选项。
func (h *Handler) Options(c *gin.Context) {
	options, err := h.service.Options(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, options)
}

// ListStockTx 查询库存变动流水。
func (h *Handler) ListStockTx(c *gin.Context) {
	var query StockTxListQuery
	if err := httpx.BindQuery(c, &query); err != nil {
		response.Fail(c, err)
		return
	}
	items, total, page, err := h.service.ListStockTx(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, response.NewPageData(items, total, page.Page, page.PageSize))
}

// CreateStockTx 登记入库 / 退料 / 报废。
func (h *Handler) CreateStockTx(c *gin.Context) {
	var req StockTxCreateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	entity, err := h.service.CreateStockTx(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, entity)
}

// ListMaterials 查询某次维修的用料明细。
func (h *Handler) ListMaterials(c *gin.Context) {
	repairID, err := httpx.ParseID(c, "repairId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	items, err := h.service.ListMaterials(c.Request.Context(), repairID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, items)
}

// ConsumptionRanking 备件消耗排名。
func (h *Handler) ConsumptionRanking(c *gin.Context) {
	limit, err := parseLimit(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	items, err := h.service.ConsumptionRanking(c.Request.Context(), limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, items)
}

// ShortageList 缺货/预警备件清单。
func (h *Handler) ShortageList(c *gin.Context) {
	limit, err := parseLimit(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	items, err := h.service.ListShortage(c.Request.Context(), limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, items)
}

func parseLimit(c *gin.Context) (int, error) {
	raw := strings.TrimSpace(c.Query("limit"))
	if raw == "" {
		return 10, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, apperr.BadRequest("limit 必须为正整数: %q", raw)
	}
	if limit > 100 {
		limit = 100
	}
	return limit, nil
}
