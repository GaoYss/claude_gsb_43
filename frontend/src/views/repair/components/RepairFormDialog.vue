<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEdit ? '编辑维修记录' : '维修记录录入(开工)'"
    width="760px"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="syncForm"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
      <el-form-item v-if="!isEdit && !lockedFault" label="关联故障" prop="fault_id">
        <el-select
          v-model="form.fault_id"
          filterable
          remote
          reserve-keyword
          :remote-method="searchFaults"
          :loading="faultLoading"
          placeholder="输入故障单号 / 路灯编号搜索未闭环故障"
          style="width: 100%"
          @change="handleFaultChange"
        >
          <el-option
            v-for="item in faultCandidates"
            :key="item.id"
            :label="`${item.fault_no} · ${item.lamp_code} · ${item.fault_type}`"
            :value="item.id"
          />
        </el-select>
      </el-form-item>

      <el-descriptions v-if="currentFault" :column="2" border size="small" class="fault-summary">
        <el-descriptions-item label="故障单号">{{ currentFault.fault_no }}</el-descriptions-item>
        <el-descriptions-item label="路灯编号">{{ currentFault.lamp_code }}</el-descriptions-item>
        <el-descriptions-item label="所在道路">{{ currentFault.road_name }}</el-descriptions-item>
        <el-descriptions-item label="故障类型">{{ currentFault.fault_type }}</el-descriptions-item>
        <el-descriptions-item label="处理状态">
          <StatusTag :dict="FAULT_STATUS" :value="currentFault.status" />
        </el-descriptions-item>
        <el-descriptions-item label="紧急程度">
          <StatusTag :dict="FAULT_LEVEL" :value="currentFault.fault_level" />
        </el-descriptions-item>
        <el-descriptions-item label="故障描述" :span="2">{{ currentFault.description || '-' }}</el-descriptions-item>
      </el-descriptions>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="维修人员" prop="repairman">
            <el-select v-model="form.repairman" filterable allow-create placeholder="选择或输入维修人员" style="width: 100%">
              <el-option v-for="item in repairmanOptions" :key="item" :label="item" :value="item" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="维修班组" prop="repair_team">
            <el-select v-model="form.repair_team" filterable allow-create clearable placeholder="选择或输入班组" style="width: 100%">
              <el-option v-for="item in teamOptions" :key="item" :label="item" :value="item" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="联系电话" prop="contact_phone">
            <el-input v-model="form.contact_phone" placeholder="选填" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="开工时间" prop="started_at">
            <el-date-picker
              v-model="form.started_at"
              type="datetime"
              value-format="YYYY-MM-DD HH:mm:ss"
              placeholder="默认取当前时间"
              style="width: 100%"
            />
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="维修内容" prop="content">
            <el-input v-model="form.content" type="textarea" :rows="2" maxlength="512" show-word-limit placeholder="例如: 更换驱动电源并复测绝缘" />
          </el-form-item>
        </el-col>
      </el-row>

      <!-- 新建开工: 选择领用备件, 保存时自动扣减库存, 库存不足将拦截开工 -->
      <el-form-item v-if="!isEdit" label="领用备件">
        <div class="material-block">
          <MaterialLinesEditor v-model="form.material_lines" :part-options="partOptions" />
          <div class="form-hint text-muted">开工时按领用数量自动扣减库存; 若备件库存不足, 系统会拦住开工并提示可用数量。未用完的备件完工后可在库存台账办理退料。</div>
        </div>
      </el-form-item>

      <!-- 编辑已开工记录: 只读展示已领用明细, 退料/报废请在出入库流程中办理 -->
      <el-form-item v-else-if="issuedMaterials.length" label="已领备件">
        <el-table :data="issuedMaterials" size="small" border>
          <el-table-column prop="part_code" label="编号" width="100" />
          <el-table-column prop="part_name" label="备件名称" min-width="140" show-overflow-tooltip />
          <el-table-column label="领用" width="80">
            <template #default="{ row }">{{ row.quantity }} {{ row.unit }}</template>
          </el-table-column>
          <el-table-column label="已退" width="70">
            <template #default="{ row }">{{ row.returned_qty }} {{ row.unit }}</template>
          </el-table-column>
          <el-table-column label="已报废" width="80">
            <template #default="{ row }">{{ row.scrapped_qty }} {{ row.unit }}</template>
          </el-table-column>
        </el-table>
      </el-form-item>

      <el-alert
        v-if="shortageDetails.length"
        type="error"
        :closable="false"
        show-icon
        class="shortage-alert"
        title="备件库存不足, 无法开工"
        :description="shortageText"
      />

      <el-row :gutter="16">
        <el-col :span="16">
          <el-form-item label="耗材备注" prop="materials">
            <el-input v-model="form.materials" placeholder="其它未入台账的耗材说明, 例如 扎带若干" />
          </el-form-item>
        </el-col>
        <el-col :span="8">
          <el-form-item label="费用(元)" prop="cost">
            <el-input-number v-model="form.cost" :min="0" :precision="2" :step="10" style="width: 100%" />
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="备注" prop="remark">
            <el-input v-model="form.remark" type="textarea" :rows="2" maxlength="255" show-word-limit />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>

    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存并开工</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import StatusTag from '@/components/common/StatusTag.vue'
import MaterialLinesEditor from './MaterialLinesEditor.vue'
import { faultApi } from '@/api/fault'
import { repairApi } from '@/api/repair'
import { inventoryApi } from '@/api/inventory'
import { useDictStore } from '@/stores/dict'
import { FAULT_LEVEL, FAULT_STATUS } from '@/constants/dict'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  model: { type: Object, default: null },
  fault: { type: Object, default: null },
})

const emit = defineEmits(['update:modelValue', 'saved'])

const dictStore = useDictStore()
const formRef = ref(null)
const submitting = ref(false)
const faultLoading = ref(false)
const faultCandidates = ref([])
const selectedFault = ref(null)
const partOptions = ref([])
const shortageDetails = ref([])

const isEdit = computed(() => Boolean(props.model?.id))
const lockedFault = computed(() => Boolean(props.fault?.id))
const currentFault = computed(() => selectedFault.value ?? props.fault ?? null)
const repairmanOptions = computed(() => dictStore.repairMeta.repairmen ?? [])
const teamOptions = computed(() => dictStore.repairMeta.teams ?? [])
const issuedMaterials = computed(() => props.model?.materials_detail ?? [])
const shortageText = computed(() =>
  shortageDetails.value
    .map((item) => `${item.part_name}(${item.part_code}) 需 ${item.need} ${item.unit}, 当前可用 ${item.available} ${item.unit}`)
    .join('；'),
)

const createForm = () => ({
  fault_id: undefined,
  repairman: '',
  repair_team: '',
  contact_phone: '',
  started_at: '',
  content: '',
  materials: '',
  material_lines: [],
  cost: 0,
  remark: '',
})

const form = reactive(createForm())

const rules = {
  fault_id: [{ required: true, message: '请选择关联故障', trigger: 'change' }],
  repairman: [{ required: true, message: '请选择或输入维修人员', trigger: 'change' }],
}

async function searchFaults(keyword = '') {
  faultLoading.value = true
  try {
    const data = await faultApi.list({ keyword, only_open: true, page: 1, page_size: 20 }, { silent: true })
    faultCandidates.value = data?.items ?? []
  } catch (error) {
    faultCandidates.value = []
  } finally {
    faultLoading.value = false
  }
}

async function loadPartOptions() {
  try {
    const data = await inventoryApi.partOptions()
    partOptions.value = data?.items ?? []
  } catch (error) {
    partOptions.value = []
  }
}

function handleFaultChange(id) {
  selectedFault.value = faultCandidates.value.find((item) => item.id === id) ?? null
}

// 打开弹窗时初始化: 编辑模式回填记录, 新增模式可带入选中的故障。
async function syncForm() {
  Object.assign(form, createForm())
  selectedFault.value = null
  shortageDetails.value = []
  loadPartOptions()

  if (props.model) {
    Object.assign(form, {
      fault_id: props.model.fault_id,
      repairman: props.model.repairman,
      repair_team: props.model.repair_team,
      contact_phone: props.model.contact_phone,
      started_at: props.model.started_at ? props.model.started_at.replace('T', ' ').slice(0, 19) : '',
      content: props.model.content,
      materials: props.model.materials,
      cost: Number(props.model.cost ?? 0),
      remark: props.model.remark,
    })
    try {
      selectedFault.value = await faultApi.detail(props.model.fault_id, { silent: true })
    } catch (error) {
      selectedFault.value = null
    }
    return
  }

  if (props.fault) {
    form.fault_id = props.fault.id
    selectedFault.value = props.fault
    return
  }

  await searchFaults('')
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    if (isEdit.value) {
      const payload = {
        repairman: form.repairman,
        repair_team: form.repair_team,
        contact_phone: form.contact_phone,
        started_at: form.started_at || undefined,
        content: form.content,
        materials: form.materials,
        cost: form.cost,
        remark: form.remark,
      }
      if (!payload.started_at) delete payload.started_at
      await repairApi.update(props.model.id, payload)
      ElMessage.success('维修记录已更新')
    } else {
      const materialLines = (form.material_lines ?? [])
        .filter((line) => line.part_id && line.quantity > 0)
        .map((line) => ({ part_id: line.part_id, quantity: line.quantity }))
      const duplicate = new Set()
      for (const line of materialLines) {
        if (duplicate.has(line.part_id)) {
          ElMessage.warning('同一种备件只能填写一行, 请合并领用数量')
          submitting.value = false
          return
        }
        duplicate.add(line.part_id)
      }
      const payload = { ...form, material_lines: materialLines }
      if (!payload.started_at) delete payload.started_at
      try {
        await repairApi.create(payload)
      } catch (error) {
        shortageDetails.value = error.details ?? []
        throw error
      }
      ElMessage.success('维修记录已录入, 备件已出库, 故障状态更新为维修中')
    }
    emit('update:modelValue', false)
    emit('saved')
  } catch (error) {
    // 缺货等错误提示由请求拦截器统一处理, 弹窗保持打开以便调整数量
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.fault-summary {
  margin-bottom: 16px;
}

.material-block {
  width: 100%;
}

.form-hint {
  font-size: 12px;
  line-height: 1.6;
  margin-top: 6px;
}

.shortage-alert {
  margin: 0 0 16px 100px;
}
</style>
