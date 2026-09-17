<template>
  <div>
    <div class="toolbar">
      <div>
        <h2 class="page-title">
          拼合组 {{ group?.code || '' }}
          <span v-if="group" class="tag" :class="group.status === 'open' ? 'ok' : 'closed'">
            {{ group.status === 'open' ? '进行中' : '已关闭' }}
          </span>
        </h2>
        <p class="page-sub">{{ group?.title }}</p>
      </div>
      <div class="toolbar-actions">
        <button
          v-if="group && group.status === 'open'"
          class="btn"
          @click="closeGroup"
        >关闭拼合组</button>
        <router-link class="btn secondary" to="/join-groups">返回列表</router-link>
      </div>
    </div>

    <p v-if="error" class="error">{{ error }}</p>

    <div v-if="group" class="card info">
      <p><strong>备注：</strong>{{ group.note || '无' }}</p>
      <p><strong>创建时间：</strong>{{ formatDate(group.createdAt) }}　<strong>成员数：</strong>{{ members.length }}</p>
      <p v-if="group.status === 'closed'" class="closed-hint">该组已关闭，禁止再增删成员。</p>
    </div>

    <div v-if="group" class="card">
      <h3 class="section-title">组成员（残片文物）</h3>
      <table class="table">
        <thead>
          <tr>
            <th>登记号</th>
            <th>探方</th>
            <th>器物类型</th>
            <th>材质</th>
            <th>完整度</th>
            <th>描述</th>
            <th v-if="group.status === 'open'">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="m in members" :key="m.id">
            <td>{{ m.find?.registerNo }}</td>
            <td>{{ unitLabel(m.find?.unit) }}</td>
            <td><span class="tag">{{ m.find?.artifactType }}</span></td>
            <td>{{ m.find?.materialName || m.find?.material?.name || '-' }}</td>
            <td>{{ m.find?.completeness || '-' }}</td>
            <td>{{ m.find?.description || '-' }}</td>
            <td v-if="group.status === 'open'">
              <button class="btn danger small" @click="removeMember(m)">移出</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!members.length" class="page-sub">组内暂无成员</p>
    </div>

    <div v-if="group && group.status === 'open'" class="card">
      <h3 class="section-title">添加同探方残片</h3>
      <div class="filters">
        <label>
          选择探方
          <select v-model.number="candidateUnitId" @change="loadCandidates">
            <option v-for="u in units" :key="u.id" :value="u.id">
              {{ u.site?.name || '' }} / {{ u.code }}
            </option>
          </select>
        </label>
      </div>
      <p class="page-sub hint">仅列出该探方下「残缺 / 碎片」且未加入其他进行中拼合组的文物；完整器物不能入组。</p>
      <div v-if="candidates.length" class="candidate-list">
        <label v-for="f in candidates" :key="f.id" class="candidate">
          <input type="checkbox" :value="f.id" v-model="selected" />
          <span class="candidate-main">
            <strong>{{ f.registerNo }}</strong>
            <span class="tag">{{ f.artifactType }}</span>
            <span>{{ f.completeness }}</span>
          </span>
          <span class="candidate-desc">{{ f.description || '无描述' }}</span>
        </label>
      </div>
      <p v-else class="page-sub">该探方下暂无可加入的残片</p>
      <p v-if="addError" class="error">{{ addError }}</p>
      <div class="modal-actions">
        <button class="btn" :disabled="!selected.length || adding" @click="addSelected">
          {{ adding ? '加入中…' : `加入选中残片（${selected.length}）` }}
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
const candidates = ref([])
const selected = ref([])
const candidateUnitId = ref(0)
const error = ref('')
const addError = ref('')
const adding = ref(false)

const members = computed(() => group.value?.members || [])

function formatDate(v) {
  if (!v) return '-'
  return String(v).slice(0, 10)
}

function unitLabel(unit) {
  if (!unit) return '-'
  return unit.site?.name ? `${unit.site.name} / ${unit.code}` : unit.code
}

async function loadGroup() {
  const { data } = await api.get(`/joingroups/${groupId}`)
  group.value = data
}

async function loadUnits() {
  const { data } = await api.get('/units')
  units.value = data
  // 默认选中第一个成员所在探方，否则第一个探方
  const memberUnitId = members.value[0]?.find?.unitId
  candidateUnitId.value = memberUnitId || data[0]?.id || 0
}

async function loadCandidates() {
  selected.value = []
  addError.value = ''
  if (!candidateUnitId.value) {
    candidates.value = []
    return
  }
  const { data } = await api.get('/finds', { params: { unitId: candidateUnitId.value } })
  const memberIds = new Set(members.value.map((m) => m.findId))
  candidates.value = data.filter(
    (f) => f.completeness !== '完整' && !f.joinGroupId && !memberIds.has(f.id)
  )
}

async function reload() {
  await loadGroup()
  await loadCandidates()
}

async function addSelected() {
  addError.value = ''
  adding.value = true
  try {
    for (const findId of selected.value) {
      await api.post(`/joingroups/${groupId}/members`, { findId })
    }
    await reload()
  } catch (e) {
    addError.value = e.response?.data?.error || '加入失败'
    await reload()
  } finally {
    adding.value = false
  }
}

async function removeMember(m) {
  if (!confirm(`确认将「${m.find?.registerNo}」移出拼合组？`)) return
  try {
    await api.delete(`/joingroups/${groupId}/members/${m.findId}`)
    await reload()
  } catch (e) {
    alert(e.response?.data?.error || '移出失败')
  }
}

async function closeGroup() {
  if (!confirm(`确认关闭拼合组「${group.value.code}」？关闭后禁止再增删成员。`)) return
  try {
    await api.post(`/joingroups/${groupId}/close`)
    await loadGroup()
  } catch (e) {
    alert(e.response?.data?.error || '关闭失败')
  }
}

onMounted(async () => {
  error.value = ''
  try {
    await loadGroup()
    await loadUnits()
    await loadCandidates()
  } catch (e) {
    error.value = e.response?.data?.error || '加载失败'
  }
})
</script>

<style scoped>
.toolbar-actions {
  display: flex;
  gap: 0.6rem;
  align-items: center;
}

.card {
  margin-bottom: 1rem;
}

.info p {
  margin: 0.3rem 0;
}

.closed-hint {
  color: var(--danger);
}

.section-title {
  margin: 0 0 0.8rem;
  font-size: 1.05rem;
}

.filters {
  display: flex;
  gap: 1rem;
  margin-bottom: 0.6rem;
  flex-wrap: wrap;
}

.filters label {
  min-width: 260px;
}

.hint {
  margin-bottom: 0.8rem;
}

.candidate-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.candidate {
  flex-direction: row;
  align-items: center;
  gap: 0.6rem;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text);
  cursor: pointer;
}

.candidate-main {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  min-width: 260px;
}

.candidate-desc {
  color: var(--muted);
  font-size: 0.88rem;
}

.tag.ok {
  background: #dcebe2;
  color: var(--ok);
}

.tag.closed {
  background: #e4ddd2;
  color: var(--muted);
}
</style>
