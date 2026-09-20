<template>
  <div class="page">
    <PageHeader title="出入库流水" description="每一次入库、领用、退料与报废都登记在册, 退料与报废必须填写原因" />

    <el-card shadow="never">
      <div class="filter-bar">
        <el-input v-model="query.keyword" placeholder="流水单号 / 备件编号 / 名称" clearable @keyup.enter="handleSearch" />
        <el-select v-model="query.tx_type" placeholder="出入库类型" clearable @change="handleSearch">
          <el-option v-for="(item, key) in STOCK_TX_TYPE" :key="key" :label="item.label" :value="key" />
        </el-select>
        <el-input v-model="query.repair_no" placeholder="关联维修单号" clearable @keyup.enter="handleSearch" />
        <el-input v-model="query.fault_no" placeholder="关联故障单号" clearable @keyup.enter="handleSearch" />
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
        />
        <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
        <el-button :icon="RefreshLeft" @click="handleReset">重置</el-button>
      </div>
    </el-card>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="rows" stripe>
        <el-table-column prop="tx_no" label="流水单号" width="150" fixed="left" />
        <el-table-column label="类型" width="100">
          <template #default="{ row }"><StatusTag :dict="STOCK_TX_TYPE" :value="row.tx_type" /></template>
        </el-table-column>
        <el-table-column prop="part_code" label="备件编号" width="100" />
        <el-table-column prop="part_name" label="备件名称" min-width="140" show-overflow-tooltip />
        <el-table-column prop="spec" label="规格型号" min-width="130" show-overflow-tooltip />
        <el-table-column label="数量" width="100">
          <template #default="{ row }">
            <span :class="signedClass(row.stock_delta)">{{ row.stock_delta > 0 ? '+' : '' }}{{ row.stock_delta }} {{ row.unit }}</span>
          </template>
        </el-table-column>
        <el-table-column label="变动后库存" width="100">
          <template #default="{ row }">{{ row.stock_after }} {{ row.unit }}</template>
        </el-table-column>
        <el-table-column prop="reason" label="原因/说明" min-width="200" show-overflow-tooltip />
        <el-table-column prop="repair_no" label="维修单" width="145" />
        <el-table-column prop="fault_no" label="故障单" width="140" />
        <el-table-column prop="operator" label="经办人" width="90" />
        <el-table-column label="时间" width="150">
          <template #default="{ row }">{{ formatDateTime(row.occurred_at) }}</template>
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
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { RefreshLeft, Search } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatusTag from '@/components/common/StatusTag.vue'
import DataPagination from '@/components/common/DataPagination.vue'
import { inventoryApi } from '@/api/inventory'
import { STOCK_TX_TYPE } from '@/constants/dict'
import { formatDateTime } from '@/utils/format'
import { useListPage } from '@/composables/useListPage'

const { loading, rows, total, query, load, search, reset, changePage, changePageSize } = useListPage(
  inventoryApi.listTransactions,
  { keyword: '', tx_type: '', repair_no: '', fault_no: '', start_date: '', end_date: '' },
)

const dateRange = ref([])

function signedClass(delta) {
  if (delta > 0) return 'delta-in'
  if (delta < 0) return 'delta-out'
  return 'text-muted'
}

function handleSearch() {
  query.start_date = dateRange.value?.[0] ?? ''
  query.end_date = dateRange.value?.[1] ?? ''
  search()
}

function handleReset() {
  dateRange.value = []
  reset()
}
</script>

<style scoped>
.delta-in {
  color: #67c23a;
  font-weight: 600;
}

.delta-out {
  color: #f56c6c;
  font-weight: 600;
}
</style>
