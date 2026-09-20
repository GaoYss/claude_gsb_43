<template>
  <el-drawer
    :model-value="modelValue"
    title="维修用料明细"
    size="760px"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="load"
  >
    <el-descriptions v-if="repair" :column="2" border size="small" class="repair-summary">
      <el-descriptions-item label="维修单号">{{ repair.repair_no }}</el-descriptions-item>
      <el-descriptions-item label="故障单号">{{ repair.fault_no }}</el-descriptions-item>
      <el-descriptions-item label="路灯编号">{{ repair.lamp_code }}</el-descriptions-item>
      <el-descriptions-item label="维修人员">{{ repair.repairman }}</el-descriptions-item>
    </el-descriptions>

    <el-table v-loading="loading" :data="materials" size="small" border>
      <el-table-column prop="part_code" label="备件编号" width="100" />
      <el-table-column prop="part_name" label="备件名称" min-width="130" show-overflow-tooltip />
      <el-table-column prop="spec" label="规格" min-width="120" show-overflow-tooltip />
      <el-table-column label="领用" width="80">
        <template #default="scope">{{ scope.row.quantity }} {{ scope.row.unit }}</template>
      </el-table-column>
      <el-table-column label="已退料" width="80">
        <template #default="scope">{{ scope.row.returned_qty }} {{ scope.row.unit }}</template>
      </el-table-column>
      <el-table-column label="已报废" width="80">
        <template #default="scope">{{ scope.row.scrapped_qty }} {{ scope.row.unit }}</template>
      </el-table-column>
      <el-table-column label="净消耗" width="80">
        <template #default="scope">
          <span class="consumed">{{ scope.row.quantity - scope.row.returned_qty }} {{ scope.row.unit }}</span>
        </template>
      </el-table-column>
      <el-table-column v-if="canOperate" label="操作" width="150" fixed="right">
        <template #default="scope">
          <el-button
            link
            type="success"
            :disabled="openQty(scope.row) <= 0"
            @click="openAction(scope.row, 'return')"
          >退料</el-button>
          <el-button
            link
            type="danger"
            :disabled="openQty(scope.row) <= 0"
            @click="openAction(scope.row, 'scrap')"
          >报废</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="!loading && !materials.length" description="该维修单暂未领用台账备件" :image-size="80" />

    <el-dialog
      v-model="actionVisible"
      :title="actionMode === 'return' ? '维修退料登记' : '现场报废登记'"
      width="460px"
      append-to-body
    >
      <el-form label-width="80px">
        <el-form-item label="备件">{{ actionPartText }}</el-form-item>
        <el-form-item :label="actionMode === 'return' ? '退料数量' : '报废数量'" required>
          <el-input-number v-model="actionForm.quantity" :min="1" :max="actionOpenQty" :step="1" style="width: 100%" />
        </el-form-item>
        <el-form-item :label="actionMode === 'return' ? '退料原因' : '报废原因'" required>
          <el-input
            v-model="actionForm.reason"
            type="textarea"
            :rows="3"
            maxlength="255"
            show-word-limit
            :placeholder="actionMode === 'return' ? '例如 现场判断无需更换, 备件未拆封退回' : '例如 拆卸时外壳碎裂无法复用'"
          />
        </el-form-item>
        <el-form-item label="经办人">
          <el-input v-model="actionForm.operator" placeholder="选填" maxlength="64" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="actionVisible = false">取消</el-button>
        <el-button :type="actionMode === 'return' ? 'success' : 'danger'" :loading="submitting" @click="submitAction">确认</el-button>
      </template>
    </el-dialog>
  </el-drawer>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { inventoryApi } from '@/api/inventory'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  repair: { type: Object, default: null },
})

const emit = defineEmits(['update:modelValue', 'saved'])

const loading = ref(false)
const materials = ref([])
const actionVisible = ref(false)
const actionMode = ref('return')
const actionRow = ref(null)
const submitting = ref(false)
const actionForm = reactive({ quantity: 1, reason: '', operator: '' })

const canOperate = computed(() => props.repair && props.repair.status === 'ongoing')

const actionOpenQty = computed(() => openQty(actionRow.value))

const actionPartText = computed(() => {
  const row = actionRow.value
  if (!row) return ''
  return `${row.part_name} (可操作 ${openQty(row)} ${row.unit})`
})

function openQty(row) {
  if (!row) return 0
  return Math.max(0, row.quantity - row.returned_qty - row.scrapped_qty)
}

async function load() {
  if (!props.repair || !props.repair.id) return
  loading.value = true
  try {
    materials.value = await inventoryApi.listMaterials(props.repair.id)
  } catch (error) {
    materials.value = []
  } finally {
    loading.value = false
  }
}

function openAction(row, mode) {
  actionRow.value = row
  actionMode.value = mode
  actionForm.quantity = 1
  actionForm.reason = ''
  actionForm.operator = (props.repair && props.repair.repairman) || ''
  actionVisible.value = true
}

async function submitAction() {
  if (!actionForm.reason.trim()) {
    ElMessage.warning(actionMode.value === 'return' ? '退料必须登记原因' : '报废必须登记原因')
    return
  }
  submitting.value = true
  try {
    const payload = { quantity: actionForm.quantity, reason: actionForm.reason, operator: actionForm.operator }
    if (actionMode.value === 'return') {
      await inventoryApi.returnMaterial(actionRow.value.id, payload)
      ElMessage.success('退料成功, 备件已回补库存')
    } else {
      await inventoryApi.scrapMaterial(actionRow.value.id, payload)
      ElMessage.success('现场报废已登记')
    }
    actionVisible.value = false
    await load()
    emit('saved')
  } catch (error) {
    // 错误提示由请求拦截器统一处理
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.repair-summary {
  margin-bottom: 16px;
}

.consumed {
  font-weight: 600;
}
</style>
