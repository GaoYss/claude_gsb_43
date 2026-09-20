<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEdit ? '编辑备件档案' : '新增备件'"
    width="680px"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="syncForm"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="备件编号" prop="code">
            <el-input v-model="form.code" :disabled="isEdit" placeholder="例如 BJ0001">
              <template v-if="!isEdit && nextCode" #append>
                <el-button @click="form.code = nextCode">用建议值</el-button>
              </template>
            </el-input>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="备件名称" prop="name">
            <el-input v-model="form.name" placeholder="例如 LED 驱动电源" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="备件分类" prop="category">
            <el-select v-model="form.category" filterable allow-create clearable placeholder="选择或输入分类" style="width: 100%">
              <el-option v-for="item in categoryOptions" :key="item" :label="item" :value="item" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="规格型号" prop="spec">
            <el-input v-model="form.spec" placeholder="例如 150W 恒流防水" />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="计量单位" prop="unit">
            <el-select v-model="form.unit" filterable allow-create placeholder="个/套/米" style="width: 100%">
              <el-option v-for="item in unitOptions" :key="item" :label="item" :value="item" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item v-if="!isEdit" label="期初库存" prop="initial_stock">
            <el-input-number v-model="form.initial_stock" :min="0" :step="1" style="width: 100%" />
          </el-form-item>
          <el-form-item v-else label="当前库存">
            <el-input-number :model-value="model.stock" disabled style="width: 100%" />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="安全库存" prop="safety_stock">
            <el-input-number v-model="form.safety_stock" :min="0" :step="1" style="width: 100%" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="单价(元)" prop="unit_price">
            <el-input-number v-model="form.unit_price" :min="0" :precision="2" :step="10" style="width: 100%" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="供应商" prop="supplier">
            <el-input v-model="form.supplier" placeholder="选填" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="状态" prop="status">
            <el-radio-group v-model="form.status">
              <el-radio v-for="(item, key) in PART_STATUS" :key="key" :value="key">{{ item.label }}</el-radio>
            </el-radio-group>
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="备注" prop="remark">
            <el-input v-model="form.remark" type="textarea" :rows="2" maxlength="255" show-word-limit />
          </el-form-item>
        </el-col>
      </el-row>
      <el-alert
        v-if="!isEdit"
        type="info"
        :closable="false"
        show-icon
        title="期初库存大于 0 时会自动生成一条采购入库流水; 建账后库存只能通过入库、退料、报废等出入库操作变动。"
      />
      <el-alert
        v-else
        type="warning"
        :closable="false"
        show-icon
        title="编辑档案不会调整库存数量, 请使用入库或报废登记变更库存。"
      />
    </el-form>

    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { inventoryApi } from '@/api/inventory'
import { PART_STATUS } from '@/constants/dict'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  model: { type: Object, default: null },
  options: { type: Object, default: () => ({}) },
})

const emit = defineEmits(['update:modelValue', 'saved'])

const formRef = ref(null)
const submitting = ref(false)
const isEdit = computed(() => Boolean(props.model?.id))
const nextCode = computed(() => props.options?.next_code ?? '')
const categoryOptions = computed(() => props.options?.categories ?? [])
const unitOptions = computed(() => Array.from(new Set(['个', '套', '只', '米', '卷', ...(props.options?.units ?? [])])))

const createForm = () => ({
  code: '',
  name: '',
  category: '',
  spec: '',
  unit: '个',
  initial_stock: 0,
  safety_stock: 0,
  unit_price: 0,
  supplier: '',
  status: 'active',
  remark: '',
})

const form = reactive(createForm())

const rules = {
  code: [{ required: true, message: '请输入备件编号', trigger: 'blur' }],
  name: [{ required: true, message: '请输入备件名称', trigger: 'blur' }],
}

function syncForm() {
  Object.assign(form, createForm())
  if (props.model) {
    Object.assign(form, {
      code: props.model.code,
      name: props.model.name,
      category: props.model.category,
      spec: props.model.spec,
      unit: props.model.unit,
      safety_stock: props.model.safety_stock,
      unit_price: Number(props.model.unit_price ?? 0),
      supplier: props.model.supplier,
      status: props.model.status,
      remark: props.model.remark,
    })
  } else if (nextCode.value) {
    form.code = nextCode.value
  }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    if (isEdit.value) {
      await inventoryApi.updatePart(props.model.id, { ...form })
      ElMessage.success('备件档案已更新')
    } else {
      await inventoryApi.createPart({ ...form })
      ElMessage.success('备件已创建')
    }
    emit('update:modelValue', false)
    emit('saved')
  } finally {
    submitting.value = false
  }
}
</script>
