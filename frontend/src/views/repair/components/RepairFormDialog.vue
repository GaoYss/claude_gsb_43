<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEdit ? '编辑维修记录' : '维修记录录入'"
    width="680px"
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
        <el-col :span="24">
          <el-form-item label="领用备件">
            <!-- 新增(开工): 可编辑备件清单, 保存即扣减库存 -->
            <div v-if="!isEdit" class="material-block">
              <el-table :data="materialRows" size="small" border>
                <el-table-column label="备件" min-width="240">
                  <template #default="{ row, $index }">
                    <el-select
                      v-model="row.part_id"
                      filterable
                      remote
                      reserve-keyword
                      :remote-method="(kw) => searchParts(kw)"
                      :loading="partLoading"
                      placeholder="输入编号 / 名称搜索备件"
                      style="width: 100%"
                      @change="(id) => handlePartChange(id, $index)"
                    >
                      <el-option
                        v-for="item in partOptions"
                        :key="item.id"
                        :label="`${item.code} · ${item.name}${item.specification ? ' · ' + item.specification : ''}`"
                        :value="item.id"
                      >
                        <span>{{ item.code }} · {{ item.name }}</span>
                        <span class="part-stock" :class="partStockClass(item)">
                          可用 {{ item.stock }} {{ item.unit }}
                        </span>
                      </el-option>
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="可用库存" width="110">
                  <template #default="{ row }">
                    <span v-if="row.part" :class="partStockClass(row.part)">
                      {{ row.part.stock }} {{ row.part.unit }}
                    </span>
                    <span v-else class="text-muted">-</span>
                  </template>
                </el-table-column>
                <el-table-column label="领用数量" width="140">
                  <template #default="{ row }">
                    <el-input-number v-model="row.quantity" :min="1" :step="1" size="small" style="width: 100%" />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="80">
                  <template #default="{ $index }">
                    <el-button link type="danger" @click="removeMaterial($index)">移除</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <el-button class="material-add" :icon="Plus" size="small" @click="addMaterial">添加备件</el-button>
              <div class="form-hint text-muted">开工保存时自动扣减库存; 任一备件库存不足将拦截开工并提示可用数量。</div>
            </div>
            <!-- 编辑: 已领用备件不可更改, 仅展示 -->
            <el-table v-else :data="props.model?.material_items ?? []" size="small" border>
              <el-table-column prop="part_code" label="备件编号" width="120" />
              <el-table-column prop="part_name" label="备件名称" min-width="160" />
              <el-table-column label="领用数量" width="120">
                <template #default="{ row }">{{ row.quantity }} {{ row.unit }}</template>
              </el-table-column>
              <el-table-column label="单价" width="100">
                <template #default="{ row }">{{ formatMoney(row.unit_price) }}</template>
              </el-table-column>
            </el-table>
          </el-form-item>
        </el-col>
        <el-col :span="16">
          <el-form-item label="耗材备注" prop="materials">
            <el-input v-model="form.materials" placeholder="例如: 驱动电源 1 个" />
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
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import StatusTag from '@/components/common/StatusTag.vue'
import { faultApi } from '@/api/fault'
import { repairApi } from '@/api/repair'
import { partsApi } from '@/api/parts'
import { useDictStore } from '@/stores/dict'
import { FAULT_LEVEL, FAULT_STATUS, partStockKey } from '@/constants/dict'
import { formatMoney } from '@/utils/format'

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
const partLoading = ref(false)
const partOptions = ref([])
const materialRows = ref([])

const isEdit = computed(() => Boolean(props.model?.id))
const lockedFault = computed(() => Boolean(props.fault?.id))
const currentFault = computed(() => selectedFault.value ?? props.fault ?? null)
const repairmanOptions = computed(() => dictStore.repairMeta.repairmen ?? [])
const teamOptions = computed(() => dictStore.repairMeta.teams ?? [])

const createForm = () => ({
  fault_id: undefined,
  repairman: '',
  repair_team: '',
  contact_phone: '',
  started_at: '',
  content: '',
  materials: '',
  cost: 0,
  remark: '',
})

const form = reactive(createForm())

const rules = {
  fault_id: [{ required: true, message: '请选择关联故障', trigger: 'change' }],
  repairman: [{ required: true, message: '请选择或输入维修人员', trigger: 'change' }],
}

// ---- 备件选择 ----
async function searchParts(keyword = '') {
  partLoading.value = true
  try {
    const data = await partsApi.list(
      { keyword, page: 1, page_size: 20 },
      { silent: true },
    )
    partOptions.value = data?.items ?? []
  } catch (error) {
    partOptions.value = []
  } finally {
    partLoading.value = false
  }
}

function addMaterial() {
  materialRows.value.push({ part_id: undefined, quantity: 1, part: null })
}

function removeMaterial(index) {
  materialRows.value.splice(index, 1)
}

function handlePartChange(partID, index) {
  const part = partOptions.value.find((item) => item.id === partID) ?? null
  if (materialRows.value[index]) {
    materialRows.value[index].part = part
  }
}

function partStockClass(part) {
  if (!part) return ''
  const key = partStockKey(part.stock, part.safety_stock)
  if (key === 'out') return 'stock-danger'
  if (key === 'shortage') return 'stock-warning'
  return 'stock-ok'
}

function collectMaterialItems() {
  const seen = new Set()
  const items = []
  for (const row of materialRows.value) {
    if (!row.part_id || row.quantity <= 0) continue
    if (seen.has(row.part_id)) continue
    seen.add(row.part_id)
    items.push({ part_id: row.part_id, quantity: row.quantity })
  }
  return items
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

function handleFaultChange(id) {
  selectedFault.value = faultCandidates.value.find((item) => item.id === id) ?? null
}

// 打开弹窗时初始化: 编辑模式回填记录, 新增模式可带入选中的故障。
async function syncForm() {
  Object.assign(form, createForm())
  selectedFault.value = null
  materialRows.value = []
  partOptions.value = []

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
  } else {
    await searchFaults('')
  }
  await searchParts('')
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  if (!isEdit.value) {
    // 前端先做一次库存校验, 给出友好提示; 服务端仍会强校验并拦截。
    for (const row of materialRows.value) {
      if (!row.part_id) {
        ElMessage.warning('领用备件存在未选择的行, 请先选择备件或移除该行')
        return
      }
      if (row.part && row.quantity > row.part.stock) {
        ElMessage.error(`备件 ${row.part.name} 库存不足: 需领用 ${row.quantity}${row.part.unit}, 当前可用 ${row.part.stock}${row.part.unit}`)
        return
      }
    }
  }

  submitting.value = true
  try {
    const payload = { ...form }
    if (!payload.started_at) {
      delete payload.started_at
    }
    if (isEdit.value) {
      const { fault_id: _ignored, material_items: _materials, ...rest } = payload
      await repairApi.update(props.model.id, rest)
      ElMessage.success('维修记录已更新')
    } else {
      payload.material_items = collectMaterialItems()
      await repairApi.create(payload)
      ElMessage.success('维修记录已录入, 已领用扣减备件库存, 故障状态更新为维修中')
    }
    emit('update:modelValue', false)
    emit('saved')
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

.material-add {
  margin-top: 8px;
}

.form-hint {
  font-size: 12px;
  line-height: 1.6;
  margin-top: 6px;
}

.part-stock {
  float: right;
  font-size: 12px;
  margin-left: 12px;
}

.stock-ok {
  color: var(--el-color-success);
}

.stock-warning {
  color: var(--el-color-warning);
  font-weight: 600;
}

.stock-danger {
  color: var(--el-color-danger);
  font-weight: 600;
}
</style>
