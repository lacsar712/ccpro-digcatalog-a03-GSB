<template>
  <div v-if="group">
    <div class="toolbar">
      <div>
        <div class="back">
          <router-link :to="{ name: 'join-groups' }">← 返回拼合组列表</router-link>
        </div>
        <h2 class="page-title">
          {{ group.code }}
          <span class="tag" :class="group.status">{{ group.status === 'open' ? '进行中' : '已关闭' }}</span>
        </h2>
        <p class="page-sub">{{ group.title }}</p>
      </div>
      <button v-if="group.status === 'open'" class="btn danger-soft" @click="closeGroup">关闭拼合组</button>
    </div>

    <div v-if="group.status === 'closed'" class="card notice">
      该拼合组已关闭，成员关系已冻结：不能再加入或移出残片。
    </div>

    <div class="card section">
      <h3>基本信息</h3>
      <table class="table">
        <tbody>
          <tr><th style="width:140px">编号</th><td>{{ group.code }}（全库唯一）</td></tr>
          <tr><th>标题</th><td>{{ group.title }}</td></tr>
          <tr><th>备注</th><td>{{ group.note || '-' }}</td></tr>
          <tr><th>创建时间</th><td>{{ formatDate(group.createdAt) }}</td></tr>
        </tbody>
      </table>
    </div>

    <div class="card section">
      <h3>当前成员（{{ members.length }}）</h3>
      <table class="table">
        <thead>
          <tr>
            <th>登记号</th>
            <th>探方</th>
            <th>器物类型</th>
            <th>材质</th>
            <th>完整度</th>
            <th>存放位置</th>
            <th v-if="group.status === 'open'">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in members" :key="m.id">
            <td>{{ m.find?.registerNo }}</td>
            <td>{{ m.find?.unit?.code || '-' }}</td>
            <td><span class="tag">{{ m.find?.artifactType }}</span></td>
            <td>{{ m.find?.materialName || m.find?.material?.name || '-' }}</td>
            <td>{{ m.find?.completeness || '-' }}</td>
            <td>{{ m.find?.storageLoc || '-' }}</td>
            <td v-if="group.status === 'open'">
              <button class="btn danger small" @click="removeMember(m.find?.id)">移出</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!members.length" class="page-sub">尚无成员，请从下方同探方残片中勾选加入。</p>
    </div>

    <div v-if="group.status === 'open'" class="card section">
      <h3>勾选同探方残片加入</h3>
      <p class="page-sub rule">
        规则：仅「残缺 / 碎片」文物可入组，完整器物不可拼合；一件文物同一时刻只能属于一个进行中的拼合组。
      </p>
      <div class="filters">
        <label>
          选择探方
          <select v-model="selectedUnitId" @change="loadCandidates">
            <option v-for="u in units" :key="u.id" :value="u.id">
              {{ u.site?.name || '' }} / {{ u.code }}
            </option>
          </select>
        </label>
      </div>

      <table class="table">
        <thead>
          <tr>
            <th style="width:48px">加入</th>
            <th>登记号</th>
            <th>器物类型</th>
            <th>完整度</th>
            <th>拼合归属</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="f in candidates" :key="f.id">
            <td>
              <input
                type="checkbox"
                :checked="picked.includes(f.id)"
                :disabled="!canPick(f)"
                @change="togglePick(f.id)"
              />
            </td>
            <td>{{ f.registerNo }}</td>
            <td><span class="tag">{{ f.artifactType }}</span></td>
            <td>{{ f.completeness || '-' }}</td>
            <td>
              <span v-if="memberFindIds.has(f.id)" class="muted">已在本组</span>
              <span v-else-if="f.joinGroupCode" class="muted">已属拼合组 {{ f.joinGroupCode }}</span>
              <span v-else-if="f.completeness === '完整'" class="muted">完整器不可拼合</span>
              <span v-else class="ok">可加入</span>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!candidates.length" class="page-sub">该探方下暂无可展示文物。</p>
      <p v-if="error" class="error">{{ error }}</p>
      <div class="row-actions">
        <button class="btn" :disabled="!picked.length || saving" @click="addPicked">
          {{ saving ? '提交中…' : `加入所选（${picked.length}）` }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import api from '../api/http'

const route = useRoute()
const groupId = route.params.id

const group = ref(null)
const units = ref([])
const selectedUnitId = ref(null)
const candidates = ref([])
const picked = ref([])
const error = ref('')
const saving = ref(false)

const members = computed(() => group.value?.members || [])
const memberFindIds = computed(() => new Set(members.value.map((m) => m.findId)))

function formatDate(v) {
  return v ? String(v).slice(0, 10) : '-'
}

// 只有「非本组、非完整、未属其他 open 组」的残片可勾选
function canPick(f) {
  return f.completeness !== '完整' && !f.joinGroupCode && !memberFindIds.value.has(f.id)
}

function togglePick(id) {
  const i = picked.value.indexOf(id)
  if (i >= 0) picked.value.splice(i, 1)
  else picked.value.push(id)
}

async function loadGroup() {
  const { data } = await api.get(`/join-groups/${groupId}`)
  group.value = data
  // 默认定位到首位成员所在探方，便于继续拼合同探方残片
  const firstUnitId = data.members?.[0]?.find?.unitId
  if (firstUnitId) selectedUnitId.value = firstUnitId
}

async function loadCandidates() {
  if (!selectedUnitId.value) return
  picked.value = []
  error.value = ''
  try {
    const { data } = await api.get('/finds', { params: { unitId: selectedUnitId.value } })
    candidates.value = data
  } catch (e) {
    error.value = e.response?.data?.error || '加载文物失败'
  }
}

async function addPicked() {
  saving.value = true
  error.value = ''
  try {
    for (const findId of picked.value) {
      await api.post(`/join-groups/${groupId}/members`, { findId })
    }
    picked.value = []
    await loadGroup()
    await loadCandidates()
  } catch (e) {
    error.value = e.response?.data?.error || '加入失败'
    await loadGroup()
    await loadCandidates()
  } finally {
    saving.value = false
  }
}

async function removeMember(findId) {
  if (!findId || !confirm('确认将该文物移出拼合组？')) return
  try {
    await api.delete(`/join-groups/${groupId}/members/${findId}`)
    await loadGroup()
    await loadCandidates()
  } catch (e) {
    alert(e.response?.data?.error || '移出失败')
  }
}

async function closeGroup() {
  if (!confirm('确认关闭该拼合组？关闭后不能再增删成员。')) return
  try {
    await api.post(`/join-groups/${groupId}/close`)
    await loadGroup()
  } catch (e) {
    alert(e.response?.data?.error || '关闭失败')
  }
}

onMounted(async () => {
  const [{ data: us }] = await Promise.all([api.get('/units'), loadGroup()])
  units.value = us
  if (!selectedUnitId.value) selectedUnitId.value = us[0]?.id ?? null
  await loadCandidates()
})
</script>

<style scoped>
.back {
  margin-bottom: 0.4rem;
  font-size: 0.88rem;
}
.back a {
  color: var(--accent);
}
.section {
  margin-top: 1rem;
}
.section h3 {
  margin: 0 0 0.75rem;
}
.notice {
  background: #f3ece0;
  color: var(--muted);
  margin-bottom: 1rem;
}
.rule {
  margin-bottom: 0.9rem;
}
.filters {
  display: flex;
  gap: 1rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}
.filters label {
  min-width: 260px;
}
.row-actions {
  margin-top: 1rem;
}
.muted {
  color: var(--muted);
}
.ok {
  color: var(--ok);
}
.danger-soft {
  background: #efe6d8;
  color: var(--danger);
}
button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
</style>
