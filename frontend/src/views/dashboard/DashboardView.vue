<template>
  <div v-loading="loading" class="page">
    <PageHeader title="运行看板" description="路灯台账、故障登记与维修进展的整体概览">
      <el-button :icon="Refresh" @click="load">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="$router.push('/faults')">去登记故障</el-button>
    </PageHeader>

    <div class="card-grid">
      <StatCard
        label="路灯总数"
        :value="overview.lamp.total"
        suffix="盏"
        icon="Postcard"
        color="#409eff"
        :hint="`覆盖 ${overview.lamp.road_count} 条道路`"
      />
      <StatCard
        label="未闭环故障"
        :value="overview.fault.open_total"
        suffix="条"
        icon="Warning"
        color="#e6a23c"
        :hint="`待处理 ${pendingCount} 条 / 维修中 ${processingCount} 条`"
      />
      <StatCard
        label="超期未处理"
        :value="overview.fault.overdue_total"
        suffix="条"
        icon="AlarmClock"
        color="#f56c6c"
        :hint="`超过 ${overview.overdue_threshold_hours} 小时仍未开工`"
      />
      <StatCard
        label="平均维修时长"
        :value="overview.repair.average_duration_hours"
        suffix="小时"
        icon="Timer"
        color="#67c23a"
        :hint="`今日完成维修 ${overview.repair.today_finished} 次`"
      />
      <StatCard
        label="维修费用合计"
        :value="overview.repair.total_cost"
        suffix="元"
        icon="Money"
        color="#909399"
        :hint="`累计维修记录 ${overview.repair.total} 条`"
      />
      <StatCard
        label="今日新增故障"
        :value="overview.fault.today_reported"
        suffix="条"
        icon="DataLine"
        color="#409eff"
        :hint="`故障累计 ${overview.fault.total} 条`"
      />
      <StatCard
        label="备件缺货预警"
        :value="overview.parts.shortage_kinds"
        suffix="种"
        icon="Box"
        color="#f56c6c"
        :hint="`零库存 ${overview.parts.out_of_stock_kinds} 种 / 共 ${overview.parts.total_kinds} 种`"
      />
    </div>

    <el-row :gutter="16">
      <el-col :xs="24" :md="8">
        <el-card shadow="never">
          <div class="section-title">路灯运行状态</div>
          <BarList :items="runStatusItems" />
        </el-card>
      </el-col>
      <el-col :xs="24" :md="8">
        <el-card shadow="never">
          <div class="section-title">故障处理状态</div>
          <BarList :items="faultStatusItems" />
        </el-card>
      </el-col>
      <el-col :xs="24" :md="8">
        <el-card shadow="never">
          <div class="section-title">故障类型分布</div>
          <BarList :items="overview.fault_by_type" />
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16">
      <el-col :xs="24" :md="12">
        <el-card shadow="never">
          <div class="section-title">
            <span>最近登记故障</span>
            <el-button link type="primary" @click="$router.push('/faults')">查看全部</el-button>
          </div>
          <el-table :data="overview.recent_faults" size="small" @row-click="goTrack">
            <el-table-column prop="fault_no" label="故障单号" width="140" />
            <el-table-column prop="lamp_code" label="路灯编号" width="110" />
            <el-table-column prop="road_name" label="道路" min-width="100" />
            <el-table-column prop="fault_type" label="类型" width="100" />
            <el-table-column label="状态" width="90">
              <template #default="{ row }"><StatusTag :dict="FAULT_STATUS" :value="row.status" /></template>
            </el-table-column>
            <el-table-column label="已等待" width="90">
              <template #default="{ row }">{{ formatWaiting(row.waiting_hours) }}</template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-card shadow="never">
          <div class="section-title">
            <span>超期未处理故障</span>
            <el-tag type="danger" effect="plain" size="small">超过 {{ overview.overdue_threshold_hours }} 小时</el-tag>
          </div>
          <el-table :data="overview.overdue_faults" size="small" @row-click="goTrack">
            <el-table-column prop="fault_no" label="故障单号" width="140" />
            <el-table-column prop="lamp_code" label="路灯编号" width="110" />
            <el-table-column prop="road_name" label="道路" min-width="100" />
            <el-table-column label="等级" width="90">
              <template #default="{ row }"><StatusTag :dict="FAULT_LEVEL" :value="row.fault_level" /></template>
            </el-table-column>
            <el-table-column label="已等待" width="90">
              <template #default="{ row }">{{ formatWaiting(row.waiting_hours) }}</template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never">
      <div class="section-title">故障高发道路 TOP5</div>
      <BarList :items="overview.top_roads" />
    </el-card>

    <el-row :gutter="16">
      <el-col :xs="24" :md="12">
        <el-card shadow="never">
          <div class="section-title">
            <span>备件消耗排名 TOP10</span>
            <el-button link type="primary" @click="$router.push('/parts')">库存台账</el-button>
          </div>
          <el-table :data="overview.part_consumption" size="small">
            <el-table-column type="index" label="#" width="44" />
            <el-table-column prop="part_code" label="备件编号" width="110" />
            <el-table-column prop="part_name" label="备件名称" min-width="130" show-overflow-tooltip />
            <el-table-column label="领用量" width="110">
              <template #default="{ row }">
                <span class="qty-strong">{{ row.total_qty }} {{ row.unit }}</span>
              </template>
            </el-table-column>
            <el-table-column label="涉及维修" width="90">
              <template #default="{ row }">{{ row.repair_count }} 次</template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!overview.part_consumption?.length" description="暂无领用记录" :image-size="60" />
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-card shadow="never">
          <div class="section-title">
            <span>缺货 / 库存预警</span>
            <el-tag type="danger" effect="plain" size="small">{{ overview.parts.shortage_kinds }} 种待补货</el-tag>
          </div>
          <el-table :data="overview.shortage_parts" size="small" @row-click="goParts">
            <el-table-column prop="code" label="备件编号" width="110" />
            <el-table-column prop="name" label="备件名称" min-width="130" show-overflow-tooltip />
            <el-table-column label="当前库存" width="100">
              <template #default="{ row }">
                <span :class="row.stock <= 0 ? 'stock-danger' : 'stock-warning'">
                  {{ row.stock }} {{ row.unit }}
                </span>
              </template>
            </el-table-column>
            <el-table-column label="安全库存" width="100">
              <template #default="{ row }">{{ row.safety_stock }} {{ row.unit }}</template>
            </el-table-column>
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <StatusTag :dict="PART_STOCK_STATUS" :value="partStockKey(row.stock, row.safety_stock)" />
              </template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!overview.shortage_parts?.length" description="库存充足, 暂无预警" :image-size="60" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Refresh } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatCard from '@/components/common/StatCard.vue'
import BarList from '@/components/common/BarList.vue'
import StatusTag from '@/components/common/StatusTag.vue'
import { statusApi } from '@/api/status'
import { FAULT_LEVEL, FAULT_STATUS, RUN_STATUS, PART_STOCK_STATUS, partStockKey } from '@/constants/dict'
import { formatWaiting } from '@/utils/format'

const router = useRouter()
const loading = ref(false)

const emptyOverview = () => ({
  lamp: { total: 0, road_count: 0, by_run_status: {} },
  fault: { total: 0, open_total: 0, by_status: {}, today_reported: 0, overdue_total: 0 },
  repair: { total: 0, ongoing_total: 0, finished_total: 0, today_finished: 0, average_duration_hours: 0, total_cost: 0 },
  parts: { total_kinds: 0, total_stock: 0, shortage_kinds: 0, out_of_stock_kinds: 0, total_stock_value: 0 },
  fault_by_type: [],
  fault_by_level: [],
  top_roads: [],
  recent_faults: [],
  overdue_faults: [],
  part_consumption: [],
  shortage_parts: [],
  overdue_threshold_hours: 24,
})

const overview = ref(emptyOverview())

const runStatusItems = computed(() =>
  Object.entries(RUN_STATUS).map(([key, item]) => ({
    label: item.label,
    count: overview.value.lamp.by_run_status?.[key] ?? 0,
  })),
)

const faultStatusItems = computed(() =>
  Object.entries(FAULT_STATUS).map(([key, item]) => ({
    label: item.label,
    count: overview.value.fault.by_status?.[key] ?? 0,
  })),
)

const pendingCount = computed(() => overview.value.fault.by_status?.pending ?? 0)
const processingCount = computed(() => overview.value.fault.by_status?.processing ?? 0)

async function load() {
  loading.value = true
  try {
    overview.value = await statusApi.overview()
  } catch (error) {
    overview.value = emptyOverview()
  } finally {
    loading.value = false
  }
}

function goTrack(row) {
  router.push({ path: '/status/track', query: { fault_no: row.fault_no } })
}

function goParts() {
  router.push('/parts')
}

onMounted(load)
</script>

<style scoped>
.qty-strong {
  font-weight: 600;
  color: var(--el-color-primary);
}

.stock-danger {
  color: var(--el-color-danger);
  font-weight: 600;
}

.stock-warning {
  color: var(--el-color-warning);
  font-weight: 600;
}
</style>
