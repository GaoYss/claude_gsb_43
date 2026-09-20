import request from './request'

// 备件与耗材库存接口。
export const inventoryApi = {
  // 备件台账
  listParts: (params) => request.get('/inventory/parts', { params }),
  createPart: (data) => request.post('/inventory/parts', data),
  partDetail: (id) => request.get(`/inventory/parts/${id}`),
  updatePart: (id, data) => request.put(`/inventory/parts/${id}`, data),
  removePart: (id) => request.delete(`/inventory/parts/${id}`),
  inbound: (id, data) => request.post(`/inventory/parts/${id}/inbound`, data),
  partOptions: () => request.get('/inventory/parts/options'),
  partMeta: () => request.get('/inventory/parts/meta'),
  partStatistics: () => request.get('/inventory/parts/statistics'),
  consumption: (limit = 10) => request.get('/inventory/parts/consumption', { params: { limit } }),
  shortage: (limit = 20) => request.get('/inventory/parts/shortage', { params: { limit } }),

  // 出入库流水
  listTransactions: (params) => request.get('/inventory/transactions', { params }),

  // 维修用料明细
  listMaterials: (repairId) => request.get(`/inventory/materials/repair/${repairId}`),
  returnMaterial: (id, data) => request.post(`/inventory/materials/${id}/return`, data),
  scrapMaterial: (id, data) => request.post(`/inventory/materials/${id}/scrap`, data),
}
