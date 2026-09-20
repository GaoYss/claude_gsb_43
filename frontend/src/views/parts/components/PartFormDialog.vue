<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEdit ? '编辑备件' : '新增备件'"
    width="720px"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="syncForm"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="备件编号" prop="code">
            <el-input v-model="form.code" placeholder="例如 BJ-00001" :disabled="isEdit" />
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
          <el-form-item label="规格型号" prop="specification">
            <el-input v-model="form.specification" placeholder="例如 150W 恒流防水" />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="计量单位" prop="unit">
            <el-input v-model="form.unit" placeholder="个 / 套 / 米" />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item v-if="!isEdit" label="期初库存" prop="stock">
            <el-input-number v-model="form.stock" :min="0" :step="1" style="width: 100%" />
          </el-form-item>
          <el-form-item v-else label="当前库存">
            <el-input :model-value="form.stock" disabled />
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
        <el-col :span="24">
          <el-form-item label="库位" prop="location">
            <el-input v-model="form.location" placeholder="例如 A 区 01 架" />
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
        title="期初库存大于 0 时会自动登记一条入库流水; 之后库存只能通过入库/退料/报废等流水变动。"
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
import { partsApi } from '@/api/parts'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  model: { type: Object, default: null },
  categoryOptions: { type: Array, default: () => [] },
})

const emit = defineEmits(['update:modelValue', 'saved'])

const formRef = ref(null)
const submitting = ref(false)
const isEdit = computed(() => Boolean(props.model?.id))

const createForm = () => ({
  code: '',
  name: '',
  category: '',
  specification: '',
  unit: '个',
  stock: 0,
  safety_stock: 0,
  unit_price: 0,
  supplier: '',
  location: '',
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
      specification: props.model.specification,
      unit: props.model.unit,
      stock: props.model.stock,
      safety_stock: props.model.safety_stock,
      unit_price: Number(props.model.unit_price ?? 0),
      supplier: props.model.supplier,
      location: props.model.location,
      remark: props.model.remark,
    })
  }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    if (isEdit.value) {
      await partsApi.update(props.model.id, { ...form })
      ElMessage.success('备件已更新')
    } else {
      await partsApi.create({ ...form })
      ElMessage.success('备件已创建')
    }
    emit('update:modelValue', false)
    emit('saved')
  } finally {
    submitting.value = false
  }
}
</script>
