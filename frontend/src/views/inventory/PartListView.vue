<template>
  <div class="page">
    <PageHeader title="备件与耗材库存" description="备件台账独立管理, 维修开工自动扣减库存, 出入库全程留痕">
      <el-button :icon="Refresh" @click="reloadAll">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="openCreate">新增备件</el-button>
    </PageHeader>

    <div class="card-grid">
      <StatCard label="备件品种" :value="statistics.total" suffix="种" icon="Box" color="#409eff" :hint="`在用 ${statistics.active_total} / 停用 ${statistics.inactive_total}`" />
      <StatCard label="缺货预警" :value="statistics.shortage_total" suffix="种" icon="Warning" color="#f56c6c" :hint="`库存低于安全库存 ${statistics.shortage_total} 种`" />
      <StatCard label="在库总量" :value="statistics.total_stock_qty" icon="Files" color="#e6a23c" hint="全部备件库存数量合计" />
      <StatCard label="在库金额" :value="statistics.total_stock_value" suffix="元" icon="Money" color="#67c23a" hint="按当前移动加权单价估算" />
    </div>

    <el-card shadow="never">
      <div class="filter-bar">
        <el-input v-model="query.keyword" placeholder="编号 / 名称 / 规格 / 供应商" clearable @keyup.enter="handleSearch" />
        <el-select v-model="query.category" placeholder="备件分类" clearable @change="handleSearch">
          <el-option v-for="item in options.categories" :key="item" :label="item" :value="item" />
        </el-select>
        <el-select v-model="query.status" placeholder="状态" clearable @change="handleSearch">
          <el-option v-for="(item, key) in PART_STATUS" :key="key" :label="item.label" :value="key" />
        </el-select>
        <el-checkbox v-model="query.shortage" :true-label="true" :false-label="false" @change="handleSearch">仅看缺货</el-checkbox>
        <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
        <el-button :icon="RefreshLeft" @click="handleReset">重置</el-button>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="code" label="备件编号" width="110" fixed="left" />
        <el-table-column prop="name" label="备件名称" min-width="150" show-overflow-tooltip />
        <el-table-column prop="category" label="分类" width="100" />
        <el-table-column prop="spec" label="规格型号" min-width="150" show-overflow-tooltip />
        <el-table-column label="当前库存" width="130">
          <template #default="{ row }">
            <span :class="{ 'stock-danger': row.stock <= row.safety_stock }" class="stock-cell">
              {{ row.stock }} {{ row.unit }}
              <el-icon v-if="row.stock <= row.safety_stock" color="#f56c6c"><Warning /></el-icon>
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="safety_stock" label="安全库存" width="100">
          <template #default="{ row }">{{ row.safety_stock }} {{ row.unit }}</template>
        </el-table-column>
        <el-table-column label="单价" width="100">
          <template #default="{ row }">{{ formatMoney(row.unit_price) }}</template>
        </el-table-column>
        <el-table-column prop="supplier" label="供应商" min-width="120" show-overflow-tooltip />
        <el-table-column label="状态" width="80">
          <template #default="{ row }"><StatusTag :dict="PART_STATUS" :value="row.status" /></template>
        </el-table-column>
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openInbound(row)">入库</el-button>
            <el-button link type="warning" @click="openScrap(row)">报废</el-button>
            <el-button link type="primary" @click="openTransactions(row)">流水</el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <DataPagination
        :page="query.page"
        :page-size="query.page_size"
        :total="total"
        @page-change="changePage"
        @size-change="changePageSize"
      />
    </el-card>

    <PartFormDialog v-model="formVisible" :model="editing" :options="options" @saved="handleSaved" />
    <StockActionDialog v-model="actionVisible" :part="actionPart" :mode="actionMode" @saved="handleSaved" />
    <TransactionDrawer v-model="txnVisible" :part="txnPart" />
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, RefreshLeft, Search, Warning } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatCard from '@/components/common/StatCard.vue'
import StatusTag from '@/components/common/StatusTag.vue'
import DataPagination from '@/components/common/DataPagination.vue'
import PartFormDialog from './components/PartFormDialog.vue'
import StockActionDialog from './components/StockActionDialog.vue'
import TransactionDrawer from './components/TransactionDrawer.vue'
import { inventoryApi } from '@/api/inventory'
import { PART_STATUS } from '@/constants/dict'
import { formatMoney } from '@/utils/format'
import { useListPage } from '@/composables/useListPage'

const { loading, rows, total, query, load, search, reset, changePage, changePageSize } = useListPage(
  inventoryApi.listParts,
  { keyword: '', category: '', status: '', shortage: false },
)

const statistics = ref({ total: 0, active_total: 0, inactive_total: 0, shortage_total: 0, total_stock_qty: 0, total_stock_value: 0 })
const options = ref({ items: [], categories: [], units: [], next_code: '' })

const formVisible = ref(false)
const actionVisible = ref(false)
const txnVisible = ref(false)
const editing = ref(null)
const actionPart = ref(null)
const actionMode = ref('inbound')
const txnPart = ref(null)

async function loadStatistics() {
  try {
    statistics.value = await inventoryApi.partStatistics()
  } catch (error) {
    // 错误提示由请求拦截器统一处理
  }
}

async function loadOptions() {
  try {
    options.value = await inventoryApi.partOptions()
  } catch (error) {
    // 错误提示由请求拦截器统一处理
  }
}

function reloadAll() {
  load()
  loadStatistics()
  loadOptions()
}

function handleSearch() {
  search()
}

function handleReset() {
  reset()
}

function openCreate() {
  editing.value = null
  formVisible.value = true
}

function openEdit(row) {
  editing.value = { ...row }
  formVisible.value = true
}

function openInbound(row) {
  actionPart.value = { ...row }
  actionMode.value = 'inbound'
  actionVisible.value = true
}

function openScrap(row) {
  actionPart.value = { ...row }
  actionMode.value = 'scrap'
  actionVisible.value = true
}

function openTransactions(row) {
  txnPart.value = { ...row }
  txnVisible.value = true
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确认删除备件 ${row.code} ${row.name}? 有库存或出入库流水的备件无法删除。`, '删除确认', {
      type: 'warning',
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
    })
  } catch (error) {
    return
  }
  try {
    await inventoryApi.removePart(row.id)
    ElMessage.success('备件已删除')
    handleSaved()
  } catch (error) {
    // 错误提示由请求拦截器统一处理
  }
}

function handleSaved() {
  reloadAll()
}

const route = useRoute()

onMounted(() => {
  if (route.query.shortage) {
    query.shortage = true
  }
  reloadAll()
})
</script>

<style scoped>
.stock-cell {
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.stock-danger {
  color: #f56c6c;
}
</style>
