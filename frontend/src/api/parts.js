import request from './request'

// 备件与耗材库存接口。
export const partsApi = {
  list: (params) => request.get('/parts', { params }),
  detail: (id) => request.get(`/parts/${id}`),
  create: (data) => request.post('/parts', data),
  update: (id, data) => request.put(`/parts/${id}`, data),
  remove: (id) => request.delete(`/parts/${id}`),
  options: () => request.get('/parts/options'),
  statistics: () => request.get('/parts/statistics'),
  shortage: (limit = 10) => request.get('/parts/shortage', { params: { limit } }),
  consumptionRanking: (limit = 10) => request.get('/parts/consumption-ranking', { params: { limit } }),

  listTransactions: (params) => request.get('/parts/transactions', { params }),
  createTransaction: (data) => request.post('/parts/transactions', data),

  listMaterials: (repairId) => request.get(`/parts/materials/repair/${repairId}`),
}
