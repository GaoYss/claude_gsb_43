<template>
  <el-dialog
    :model-value="modelValue"
    :title="mode === 'inbound' ? '备件入库登记' : '备件报废登记'"
    width="520px"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="syncForm"
  >
    <el-descriptions v-if="part" :column="2" border size="small" class="part-summary">
      <el-descriptions-item label="备件编号">{{ part.code }}</el-descriptions-item>
      <el-descriptions-item label="备件名称">{{ part.name }}</el-descriptions-item>
      <el-descriptions-item label="规格型号">{{ part.spec || '-' }}</el-descriptions-item>
      <el-descriptions-item label="当前库存">{{ part.stock }} {{ part.unit }}</el-descriptions-item>
    </el-descriptions>

    <el-form ref="formRef" :model="form" :rules="rules" label-width="90px">
      <el-form-item :label="mode === 'inbound' ? '入库数量' : '报废数量'" prop="quantity">
        <el-input-number v-model="form.quantity" :min="1" :step="1" style="width: 100%" />
        <div v-if="part" class="form-hint text-muted">
          <template v-if="mode === 'inbound'">入库后库存: {{ part.stock + (form.quantity || 0) }} {{ part.unit }}</template>
          <template v-else>报废后库存: {{ Math.max(0, part.stock - (form.quantity || 0)) }} {{ part.unit }}, 报废数量不能超过当前库存</template>
        </div>
      </el-form-item>
      <el-form-item v-if="mode === 'inbound'" label="入库单价" prop="unit_price">
        <el-input-number v-model="form.unit_price" :min="0" :precision="2" :step="10" style="width: 100%" />
        <div class="form-hint text-muted">不填则沿用当前单价; 填写后按移动加权平均法刷新备件单价</div>
      </el-form-item>
      <el-form-item v-if="mode === 'inbound'" label="供应商" prop="supplier">
        <el-input v-model="form.supplier" placeholder="选填, 不填保留原供应商" />
      </el-form-item>
      <el-form-item label="经办人" prop="operator">
        <el-input v-model="form.operator" placeholder="选填" maxlength="64" />
      </el-form-item>
      <el-form-item :label="mode === 'inbound' ? '入库说明' : '报废原因'" prop="reason">
        <el-input
          v-model="form.reason"
          type="textarea"
          :rows="3"
          maxlength="255"
          show-word-limit
          :placeholder="mode === 'inbound' ? '选填, 例如 日常采购补货' : '必填, 例如 受潮锈蚀无法使用'"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button :type="mode === 'inbound' ? 'primary' : 'danger'" :loading="submitting" @click="handleSubmit">
        {{ mode === 'inbound' ? '确认入库' : '确认报废' }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { inventoryApi } from '@/api/inventory'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  part: { type: Object, default: null },
  mode: { type: String, default: 'inbound' }, // inbound | scrap
})

const emit = defineEmits(['update:modelValue', 'saved'])

const formRef = ref(null)
const submitting = ref(false)

const createForm = () => ({ quantity: 1, unit_price: undefined, supplier: '', operator: '', reason: '' })
const form = reactive(createForm())

const rules = {
  quantity: [{ required: true, message: '请填写数量', trigger: 'blur' }],
  reason: props.mode === 'scrap' ? [{ required: true, message: '报废必须登记原因', trigger: 'blur' }] : [],
}

watch(
  () => props.mode,
  () => Object.assign(form, createForm()),
)

function syncForm() {
  Object.assign(form, createForm())
  if (props.part) {
    form.unit_price = Number(props.part.unit_price ?? 0)
    form.supplier = props.part.supplier ?? ''
  }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  if (props.mode === 'scrap' && !form.reason.trim()) {
    ElMessage.warning('报废必须登记原因')
    return
  }

  submitting.value = true
  try {
    const payload = { quantity: form.quantity, operator: form.operator, reason: form.reason }
    if (props.mode === 'inbound') {
      await inventoryApi.inbound(props.part.id, {
        ...payload,
        unit_price: form.unit_price,
        supplier: form.supplier,
      })
      ElMessage.success('入库成功, 库存与流水已更新')
    } else {
      await inventoryApi.scrapPart(props.part.id, payload)
      ElMessage.success('报废已登记, 库存与流水已更新')
    }
    emit('update:modelValue', false)
    emit('saved')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.part-summary {
  margin-bottom: 16px;
}

.form-hint {
  font-size: 12px;
  line-height: 1.6;
}
</style>
