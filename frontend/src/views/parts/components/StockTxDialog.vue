<template>
  <el-dialog
    :model-value="modelValue"
    title="库存变动登记"
    width="560px"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="syncForm"
  >
    <el-descriptions v-if="part" :column="2" border size="small" class="part-summary">
      <el-descriptions-item label="备件编号">{{ part.code }}</el-descriptions-item>
      <el-descriptions-item label="备件名称">{{ part.name }}</el-descriptions-item>
      <el-descriptions-item label="当前库存">{{ part.stock }} {{ part.unit }}</el-descriptions-item>
      <el-descriptions-item label="安全库存">{{ part.safety_stock }} {{ part.unit }}</el-descriptions-item>
    </el-descriptions>

    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
      <el-form-item label="变动类型" prop="type">
        <el-radio-group v-model="form.type" @change="handleTypeChange">
          <el-radio-button value="inbound">入库</el-radio-button>
          <el-radio-button value="return">退料</el-radio-button>
          <el-radio-button value="scrap">报废</el-radio-button>
        </el-radio-group>
      </el-form-item>

      <el-form-item v-if="form.type === 'return'" label="关联维修单" prop="repair_id">
        <el-select
          v-model="form.repair_id"
          filterable
          remote
          reserve-keyword
          :remote-method="searchRepairs"
          :loading="repairLoading"
          placeholder="输入维修单号 / 故障单号 / 路灯编号"
          style="width: 100%"
        >
          <el-option
            v-for="item in repairCandidates"
            :key="item.id"
            :label="`${item.repair_no} · ${item.fault_no} · ${item.lamp_code} · ${item.repairman}`"
            :value="item.id"
          />
        </el-select>
        <div class="form-hint text-muted">退料数量不能超过该维修单的领用数量。</div>
      </el-form-item>

      <el-form-item :label="form.type === 'scrap' ? '报废数量' : form.type === 'return' ? '退料数量' : '入库数量'" prop="quantity">
        <el-input-number v-model="form.quantity" :min="1" :step="1" style="width: 100%" />
      </el-form-item>

      <el-form-item :label="reasonLabel" prop="reason">
        <el-input
          v-model="form.reason"
          type="textarea"
          :rows="3"
          maxlength="255"
          show-word-limit
          :placeholder="reasonPlaceholder"
        />
      </el-form-item>

      <el-form-item label="经办人" prop="operator">
        <el-input v-model="form.operator" maxlength="64" placeholder="选填, 退料默认取维修人员" />
      </el-form-item>

      <el-form-item label="发生时间" prop="occurred_at">
        <el-date-picker
          v-model="form.occurred_at"
          type="datetime"
          value-format="YYYY-MM-DD HH:mm:ss"
          placeholder="默认取当前时间"
          style="width: 100%"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">提交</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { partsApi } from '@/api/parts'
import { repairApi } from '@/api/repair'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  part: { type: Object, default: null },
  defaultType: { type: String, default: 'inbound' },
})

const emit = defineEmits(['update:modelValue', 'saved'])

const formRef = ref(null)
const submitting = ref(false)
const repairLoading = ref(false)
const repairCandidates = ref([])

const createForm = () => ({
  type: 'inbound',
  quantity: 1,
  reason: '',
  operator: '',
  occurred_at: '',
  repair_id: undefined,
})

const form = reactive(createForm())

const reasonLabel = computed(() => {
  if (form.type === 'return') return '退料原因'
  if (form.type === 'scrap') return '报废原因'
  return '入库说明'
})

const reasonPlaceholder = computed(() => {
  if (form.type === 'return') return '请填写退料原因(必填), 例如: 现场原部件仍可使用'
  if (form.type === 'scrap') return '请填写报废原因(必填), 例如: 运输破损 / 老化失效'
  return '选填, 例如: 采购入库'
})

const rules = computed(() => ({
  type: [{ required: true, message: '请选择变动类型', trigger: 'change' }],
  quantity: [{ required: true, message: '请填写数量', trigger: 'change' }],
  reason:
    form.type === 'return' || form.type === 'scrap'
      ? [{ required: true, message: '退料与报废必须登记原因', trigger: 'blur' }]
      : [],
  repair_id: form.type === 'return' ? [{ required: true, message: '退料必须关联维修单', trigger: 'change' }] : [],
}))

function syncForm() {
  const initialType = ['inbound', 'return', 'scrap'].includes(props.defaultType) ? props.defaultType : 'inbound'
  Object.assign(form, createForm(), { type: initialType })
  repairCandidates.value = []
  if (form.type === 'return') {
    searchRepairs('')
  }
}

function handleTypeChange(value) {
  form.reason = ''
  form.repair_id = undefined
  form.quantity = 1
  if (value === 'return') {
    searchRepairs('')
  }
}

async function searchRepairs(keyword = '') {
  repairLoading.value = true
  try {
    const data = await repairApi.list(
      { keyword, page: 1, page_size: 20 },
      { silent: true },
    )
    repairCandidates.value = data?.items ?? []
  } catch (error) {
    repairCandidates.value = []
  } finally {
    repairLoading.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    const payload = {
      part_id: props.part?.id,
      type: form.type,
      quantity: form.quantity,
      reason: form.reason,
      operator: form.operator,
      occurred_at: form.occurred_at || undefined,
    }
    if (form.type === 'return') {
      payload.repair_id = form.repair_id
      const repair = repairCandidates.value.find((item) => item.id === form.repair_id)
      if (!payload.operator) payload.operator = repair?.repairman ?? ''
    }
    await partsApi.createTransaction(payload)
    ElMessage.success('库存变动已登记')
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
