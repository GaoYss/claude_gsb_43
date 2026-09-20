<template>
  <div class="page">
    <PageHeader title="备件与耗材库存" description="独立台账管理备件库存: 维修开工自动领用扣减, 退料/报废登记原因, 库存不足拦截开工">
      <el-button :icon="Refresh" @click="reloadAll">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="openCreate">新增备件</el-button>
    </PageHeader>

    <div class="stat-grid">
      <StatCard label="备件种类" :value="stats.total_kinds" suffix="种" icon="Box" color="#409eff" hint="库存备件品类总数" />
      <StatCard label="库存总量" :value="stats.total_stock" suffix="件" icon="Files" color="#67c23a" :hint="`库存金额 ¥ ${Number(stats.total_stock_value ?? 0).toFixed(2)}`" />
      <StatCard label="库存预警" :value="stats.shortage_kinds" suffix="种" icon="WarningFilled" color="#e6a23c" hint="库存 ≤ 安全库存" />
      <StatCard label="缺货备件" :value="stats.out_of_stock_kinds" suffix="种" icon="CircleCloseFilled" color="#f56c6c" hint="当前库存为 0" />
    </div>

    <el-card shadow="never">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <!-- 备件台账 -->
        <el-tab-pane label="备件台账" name="parts">
          <div class="filter-bar">
            <el-input v-model="partQuery.keyword" placeholder="编号 / 名称 / 规格 / 供应商 / 库位" clearable @keyup.enter="searchParts" />
            <el-select v-model="partQuery.category" placeholder="备件分类" clearable @change="searchParts">
              <el-option v-for="item in categories" :key="item" :label="item" :value="item" />
            </el-select>
            <el-select v-model="partQuery.shortage" placeholder="库存状态" clearable @change="searchParts">
              <el-option label="全部" value="all" />
              <el-option label="库存预警" value="shortage" />
              <el-option label="缺货" value="out" />
            </el-select>
            <el-button type="primary" :icon="Search" @click="searchParts">查询</el-button>
            <el-button :icon="RefreshLeft" @click="resetParts">重置</el-button>
          </div>

          <el-table v-loading="partLoading" :data="partRows" stripe>
            <el-table-column prop="code" label="备件编号" width="110" fixed="left" />
            <el-table-column prop="name" label="备件名称" min-width="140" show-overflow-tooltip />
            <el-table-column prop="category" label="分类" width="90" />
            <el-table-column prop="specification" label="规格型号" min-width="140" show-overflow-tooltip />
            <el-table-column label="当前库存" width="120">
              <template #default="{ row }">
                <span :class="{ 'stock-danger': row.stock <= 0, 'stock-warning': row.stock > 0 && row.stock <= row.safety_stock }">
                  {{ row.stock }} {{ row.unit }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="安全库存" width="100">
              <template #default="{ row }">{{ row.safety_stock }} {{ row.unit }}</template>
            </el-table-column>
            <el-table-column label="库存状态" width="100">
              <template #default="{ row }">
                <StatusTag :dict="PART_STOCK_STATUS" :value="partStockKey(row.stock, row.safety_stock)" />
              </template>
            </el-table-column>
            <el-table-column label="单价" width="90">
              <template #default="{ row }">{{ formatMoney(row.unit_price) }}</template>
            </el-table-column>
            <el-table-column prop="supplier" label="供应商" min-width="110" show-overflow-tooltip />
            <el-table-column prop="location" label="库位" width="110" />
            <el-table-column label="操作" width="280" fixed="right">
              <template #default="{ row }">
                <el-button link type="success" @click="openInbound(row)">入库</el-button>
                <el-button link type="primary" @click="openReturn(row)">退料</el-button>
                <el-button link type="warning" @click="openScrap(row)">报废</el-button>
                <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
                <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <DataPagination
            :page="partQuery.page"
            :page-size="partQuery.page_size"
            :total="partTotal"
            @page-change="changePartPage"
            @size-change="changePartPageSize"
          />
        </el-tab-pane>

        <!-- 库存流水 -->
        <el-tab-pane label="库存流水" name="transactions">
          <div class="filter-bar">
            <el-input v-model="txQuery.keyword" placeholder="流水号 / 备件 / 维修单号 / 故障单号 / 经办人" clearable @keyup.enter="searchTx" />
            <el-select v-model="txQuery.type" placeholder="变动类型" clearable @change="searchTx">
              <el-option v-for="(item, key) in STOCK_TX_TYPE" :key="key" :label="item.label" :value="key" />
            </el-select>
            <el-date-picker
              v-model="txDateRange"
              type="daterange"
              value-format="YYYY-MM-DD"
              range-separator="至"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              @change="searchTx"
            />
            <el-button type="primary" :icon="Search" @click="searchTx">查询</el-button>
            <el-button :icon="RefreshLeft" @click="resetTx">重置</el-button>
          </div>

          <el-table v-loading="txLoading" :data="txRows" stripe>
            <el-table-column prop="tx_no" label="流水号" width="150" fixed="left" />
            <el-table-column label="类型" width="90">
              <template #default="{ row }"><StatusTag :dict="STOCK_TX_TYPE" :value="row.type" /></template>
            </el-table-column>
            <el-table-column prop="part_code" label="备件编号" width="110" />
            <el-table-column prop="part_name" label="备件名称" min-width="130" show-overflow-tooltip />
            <el-table-column label="数量" width="110">
              <template #default="{ row }">
                <span :class="isStockIn(row.type) ? 'qty-in' : 'qty-out'">
                  {{ isStockIn(row.type) ? '+' : '-' }}{{ row.quantity }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="变动后库存" width="110">
              <template #default="{ row }">{{ row.stock_after }}</template>
            </el-table-column>
            <el-table-column prop="repair_no" label="关联维修单" width="140">
              <template #default="{ row }">{{ row.repair_no || '-' }}</template>
            </el-table-column>
            <el-table-column prop="fault_no" label="故障单号" width="140">
              <template #default="{ row }">{{ row.fault_no || '-' }}</template>
            </el-table-column>
            <el-table-column prop="reason" label="原因 / 说明" min-width="180" show-overflow-tooltip />
            <el-table-column prop="operator" label="经办人" width="100" />
            <el-table-column label="发生时间" width="150">
              <template #default="{ row }">{{ formatDateTime(row.occurred_at) }}</template>
            </el-table-column>
          </el-table>
          <DataPagination
            :page="txQuery.page"
            :page-size="txQuery.page_size"
            :total="txTotal"
            @page-change="changeTxPage"
            @size-change="changeTxPageSize"
          />
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <PartFormDialog v-model="formVisible" :model="editing" :category-options="categories" @saved="handlePartSaved" />
    <StockTxDialog v-model="txVisible" :part="txPart" :default-type="txDefaultType" @saved="handleTxSaved" />
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, RefreshLeft, Search } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatusTag from '@/components/common/StatusTag.vue'
import StatCard from '@/components/common/StatCard.vue'
import DataPagination from '@/components/common/DataPagination.vue'
import PartFormDialog from './components/PartFormDialog.vue'
import StockTxDialog from './components/StockTxDialog.vue'
import { partsApi } from '@/api/parts'
import { PART_STOCK_STATUS, STOCK_TX_TYPE, partStockKey } from '@/constants/dict'
import { formatDateTime, formatMoney } from '@/utils/format'

const activeTab = ref('parts')

// ---- 备件台账 ----
const partLoading = ref(false)
const partRows = ref([])
const partTotal = ref(0)
const partQuery = reactive({ page: 1, page_size: 10, keyword: '', category: '', shortage: 'all' })
const categories = ref([])
const stats = ref({ total_kinds: 0, total_stock: 0, shortage_kinds: 0, out_of_stock_kinds: 0, total_stock_value: 0 })

async function loadParts() {
  partLoading.value = true
  try {
    const data = await partsApi.list({ ...partQuery })
    partRows.value = data?.items ?? []
    partTotal.value = data?.total ?? 0
  } finally {
    partLoading.value = false
  }
}

function searchParts() {
  partQuery.page = 1
  loadParts()
}

function resetParts() {
  Object.assign(partQuery, { page: 1, page_size: 10, keyword: '', category: '', shortage: 'all' })
  loadParts()
}

function changePartPage(page) {
  partQuery.page = page
  loadParts()
}

function changePartPageSize(size) {
  partQuery.page = 1
  partQuery.page_size = size
  loadParts()
}

// ---- 库存流水 ----
const txLoading = ref(false)
const txRows = ref([])
const txTotal = ref(0)
const txQuery = reactive({ page: 1, page_size: 10, keyword: '', type: '', start_date: '', end_date: '' })
const txDateRange = ref([])

async function loadTx() {
  txLoading.value = true
  try {
    const data = await partsApi.listTransactions({ ...txQuery })
    txRows.value = data?.items ?? []
    txTotal.value = data?.total ?? 0
  } finally {
    txLoading.value = false
  }
}

function searchTx() {
  txQuery.start_date = txDateRange.value?.[0] ?? ''
  txQuery.end_date = txDateRange.value?.[1] ?? ''
  txQuery.page = 1
  loadTx()
}

function resetTx() {
  txDateRange.value = []
  Object.assign(txQuery, { page: 1, page_size: 10, keyword: '', type: '', start_date: '', end_date: '' })
  loadTx()
}

function changeTxPage(page) {
  txQuery.page = page
  loadTx()
}

function changeTxPageSize(size) {
  txQuery.page = 1
  txQuery.page_size = size
  loadTx()
}

// ---- 弹窗与操作 ----
const formVisible = ref(false)
const txVisible = ref(false)
const editing = ref(null)
const txPart = ref(null)
const txDefaultType = ref('inbound')

function openCreate() {
  editing.value = null
  formVisible.value = true
}

function openEdit(row) {
  editing.value = { ...row }
  formVisible.value = true
}

function openInbound(row) {
  txPart.value = { ...row }
  txDefaultType.value = 'inbound'
  txVisible.value = true
}

function openReturn(row) {
  txPart.value = { ...row }
  txDefaultType.value = 'return'
  txVisible.value = true
}

function openScrap(row) {
  txPart.value = { ...row }
  txDefaultType.value = 'scrap'
  txVisible.value = true
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确认删除备件 ${row.code} ? 已有库存流水或维修领用的备件无法删除。`, '删除确认', {
      type: 'warning',
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
    })
  } catch (error) {
    return
  }
  await partsApi.remove(row.id)
  ElMessage.success('备件已删除')
  handlePartSaved()
}

function handlePartSaved() {
  loadParts()
  loadStats()
  loadOptions()
}

function handleTxSaved() {
  loadParts()
  loadStats()
  if (activeTab.value === 'transactions') loadTx()
}

async function loadStats() {
  try {
    stats.value = await partsApi.statistics()
  } catch (error) {
    // 忽略统计加载失败
  }
}

async function loadOptions() {
  try {
    const options = await partsApi.options()
    categories.value = options?.categories ?? []
  } catch (error) {
    categories.value = []
  }
}

function reloadAll() {
  loadParts()
  loadStats()
  loadOptions()
  if (activeTab.value === 'transactions') loadTx()
}

function handleTabChange(tab) {
  if (tab === 'transactions') loadTx()
}

function isStockIn(type) {
  return type === 'inbound' || type === 'return' || type === 'writeback'
}

onMounted(() => {
  loadParts()
  loadStats()
  loadOptions()
})
</script>

<style scoped>
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 16px;
}

@media (max-width: 1200px) {
  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.stock-danger {
  color: var(--el-color-danger);
  font-weight: 600;
}

.stock-warning {
  color: var(--el-color-warning);
  font-weight: 600;
}

.qty-in {
  color: var(--el-color-success);
  font-weight: 600;
}

.qty-out {
  color: var(--el-color-danger);
  font-weight: 600;
}
</style>
