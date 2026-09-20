<template>
  <div class="material-editor">
    <el-table :data="lines" size="small" border>
      <el-table-column label="备件" min-width="240">
        <template #default="{ row, $index }">
          <el-select
            v-model="row.part_id"
            filterable
            placeholder="选择备件"
            style="width: 100%"
            @change="(value) => handlePartChange($index, value)"
          >
            <el-option
              v-for="item in partOptions"
              :key="item.id"
              :label="`${item.code} · ${item.name}${item.spec ? ' · ' + item.spec : ''}`"
              :value="item.id"
            >
              <span>{{ item.code }} · {{ item.name }}</span>
              <span :class="item.stock <= item.safety_stock ? 'opt-stock-danger' : 'opt-stock'" class="float-right">
                可用 {{ item.stock }} {{ item.unit }}
              </span>
            </el-option>
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="领用数量" width="150">
        <template #default="{ row }">
          <el-input-number v-model="row.quantity" :min="1" :step="1" size="small" style="width: 100%" />
        </template>
      </el-table-column>
      <el-table-column label="可用库存" width="110">
        <template #default="{ row }">
          <span v-if="partMap[row.part_id]" :class="isShort(row) ? 'opt-stock-danger' : ''">
            {{ partMap[row.part_id].stock }} {{ partMap[row.part_id].unit }}
          </span>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="80" align="center">
        <template #default="{ $index }">
          <el-button link type="danger" @click="removeLine($index)">移除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-button v-if="!disabled" link type="primary" :icon="Plus" class="add-line" @click="addLine">添加备件</el-button>
    <div v-if="shortageLines.length" class="shortage-tip">
      <el-icon color="#f56c6c"><Warning /></el-icon>
      <span v-for="(line, index) in shortageLines" :key="index">
        {{ line.name }} 领用 {{ line.need }} {{ line.unit }}, 仅剩 {{ line.stock }} {{ line.unit }};
      </span>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { Plus, Warning } from '@element-plus/icons-vue'

const props = defineProps({
  modelValue: { type: Array, default: () => [] },
  partOptions: { type: Array, default: () => [] },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue'])

const lines = ref([...(props.modelValue ?? [])])

watch(
  () => props.modelValue,
  (value) => {
    const next = value ?? []
    if (next !== lines.value) {
      lines.value = [...next]
    }
  },
)

function emitChange() {
  emit('update:modelValue', lines.value)
}

function addLine() {
  lines.value.push({ part_id: undefined, quantity: 1 })
  emitChange()
}

function removeLine(index) {
  lines.value.splice(index, 1)
  emitChange()
}

function handlePartChange(index, value) {
  const part = props.partOptions.find((item) => item.id === value)
  if (part && part.stock <= 0) {
    lines.value[index].quantity = 1
  }
  emitChange()
}

const partMap = computed(() => {
  const map = {}
  for (const item of props.partOptions) {
    map[item.id] = item
  }
  return map
})

function isShort(row) {
  const part = partMap.value[row.part_id]
  return part && row.quantity > part.stock
}

const shortageLines = computed(() =>
  lines.value
    .filter((row) => row.part_id && isShort(row))
    .map((row) => {
      const part = partMap.value[row.part_id]
      return { name: part.name, need: row.quantity, stock: part.stock, unit: part.unit }
    }),
)
</script>

<style scoped>
.material-editor {
  width: 100%;
}

.add-line {
  margin-top: 8px;
}

.float-right {
  float: right;
  font-size: 12px;
}

.opt-stock {
  color: #909399;
}

.opt-stock-danger {
  color: #f56c6c;
  font-weight: 600;
}

.shortage-tip {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  padding: 8px 12px;
  background-color: #fef0f0;
  border-radius: 4px;
  color: #f56c6c;
  font-size: 12px;
}
</style>
