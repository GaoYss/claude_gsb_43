<template>
  <el-drawer
    :model-value="modelValue"
    :title="part ? `${part.name} · 出入库流水` : '出入库流水'"
    size="860px"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="load"
  >
    <el-descriptions v-if="part" :column="4" border size="small" class="part-meta">
      <el-descriptions-item label="备件编号">{{ part.code }}</el-descriptions-item>
      <el-descriptions-item label="当前库存">{{ part.stock }} {{ part.unit }}</el-descriptions-item>
      <el-descriptions-item label="安全库存">{{ part.safety_stock }} {{ part.unit }}</el-descriptions-item>
      <el-descriptions-item label="状态"><StatusTag :dict="PART_STATUS" :value="part.status" /></el-descriptions-item>
    </el-descriptions>

    <el-table v-loading="loading" :data="rows" size="small" stripe>
      <el-table-column prop="tx_no" label="流水单号" width="150" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }"><StatusTag :dict="STOCK_TX_TYPE" :value="row.tx_type" /></template>
      </el-table-column>
      <el-table-column label="数量" width="100">
        <template #default="{ row }">
          <span :class="signedClass(row.stock_delta)">{{ row.stock_delta > 0 ? '+' : '' }}{{ row.stock_delta }} {{ row.unit }}</span>
        </template>
      </el-table-column>
      <el-table-column label="变动后库存" width="100">
        <template #default="{ row }">{{ row.stock_after }} {{ row.unit }}</template>
      </el-table-column>
      <el-table-column prop="reason" label="原因/说明" min-width="180" show-overflow-tooltip />
      <el-table-column prop="repair_no" label="关联维修单" width="145">
        <template #default="{ row }">{{ row.repair_no || '-' }}</template>
      </el-table-column>
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
  </el-drawer>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
import StatusTag from '@/components/common/StatusTag.vue'
import DataPagination from '@/components/common/DataPagination.vue'
import { inventoryApi } from '@/api/inventory'
import { PART_STATUS, STOCK_TX_TYPE } from '@/constants/dict'
import { formatDateTime } from '@/utils/format'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  part: { type: Object, default: null },
})

defineEmits(['update:modelValue'])

const loading = ref(false)
const rows = ref([])
const total = ref(0)
const query = reactive({ page: 1, page_size: 10 })

function signedClass(delta) {
  if (delta > 0) return 'delta-in'
  if (delta < 0) return 'delta-out'
  return 'text-muted'
}

async function load() {
  if (!props.part) return
  loading.value = true
  try {
    const data = await inventoryApi.listTransactions({ ...query, part_id: props.part.id })
    rows.value = data?.items ?? []
    total.value = data?.total ?? 0
  } catch (error) {
    rows.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function changePage(page) {
  query.page = page
  load()
}

function changePageSize(size) {
  query.page = 1
  query.page_size = size
  load()
}

watch(() => props.part?.id, load)
</script>

<style scoped>
.part-meta {
  margin-bottom: 16px;
}

.delta-in {
  color: #67c23a;
  font-weight: 600;
}

.delta-out {
  color: #f56c6c;
  font-weight: 600;
}
</style>
