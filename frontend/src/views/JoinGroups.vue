<template>
  <div>
    <div class="toolbar">
      <div>
        <h2 class="page-title">残片拼合</h2>
        <p class="page-sub">将同探方出土的残缺/碎片文物编入拼合组；关闭后组成关系冻结</p>
      </div>
      <button class="btn" @click="openCreate">新增拼合组</button>
    </div>

    <div class="card">
      <div class="filters">
        <label>
          状态筛选
          <select v-model="filterStatus" @change="load">
            <option value="">全部</option>
            <option value="open">进行中（open）</option>
            <option value="closed">已关闭（closed）</option>
          </select>
        </label>
      </div>

      <table class="table">
        <thead>
          <tr>
            <th>拼合组编号</th>
            <th>标题</th>
            <th>状态</th>
            <th>成员数</th>
            <th>备注</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="g in list" :key="g.id">
            <td>
              <router-link class="code-link" :to="{ name: 'join-group-detail', params: { id: g.id } }">
                {{ g.code }}
              </router-link>
            </td>
            <td>{{ g.title }}</td>
            <td>
              <span class="tag" :class="g.status">{{ g.status === 'open' ? '进行中' : '已关闭' }}</span>
            </td>
            <td>{{ g.memberCount }}</td>
            <td>{{ g.note || '-' }}</td>
            <td class="ops">
              <button class="btn secondary small" @click="$router.push({ name: 'join-group-detail', params: { id: g.id } })">
                详情
              </button>
              <button v-if="g.status === 'open'" class="btn secondary small" @click="openEdit(g)">编辑</button>
              <button v-if="g.status === 'open'" class="btn small" @click="closeGroup(g)">关闭组</button>
              <button class="btn danger small" @click="remove(g)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!list.length" class="page-sub">暂无数据</p>
      <p v-if="error" class="error">{{ error }}</p>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="showModal = false">
      <div class="modal">
        <h3>{{ form.id ? '编辑拼合组' : '新增拼合组' }}</h3>
        <div class="form-grid">
          <label>
            拼合组编号（全库唯一）
            <input v-model="form.code" placeholder="如 JOIN-T1-01" />
          </label>
          <label>
            标题
            <input v-model="form.title" />
          </label>
          <label class="full">
            备注
            <textarea v-model="form.note" />
          </label>
        </div>
        <p class="page-sub">新建组默认为 open 状态；关闭操作需在详情或列表中单独执行。</p>
        <p v-if="formError" class="error">{{ formError }}</p>
        <div class="modal-actions">
          <button class="btn secondary" @click="showModal = false">取消</button>
          <button class="btn" @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import api from '../api/http'

const list = ref([])
const filterStatus = ref('')
const error = ref('')
const formError = ref('')
const showModal = ref(false)
const form = reactive({ id: null, code: '', title: '', note: '' })

async function load() {
  error.value = ''
  try {
    const params = {}
    if (filterStatus.value) params.status = filterStatus.value
    const { data } = await api.get('/join-groups', { params })
    list.value = data
  } catch (e) {
    error.value = e.response?.data?.error || '加载失败'
  }
}

function openCreate() {
  Object.assign(form, { id: null, code: '', title: '', note: '' })
  formError.value = ''
  showModal.value = true
}

function openEdit(g) {
  Object.assign(form, { id: g.id, code: g.code, title: g.title, note: g.note || '' })
  formError.value = ''
  showModal.value = true
}

async function save() {
  formError.value = ''
  try {
    const payload = { code: form.code.trim(), title: form.title.trim(), note: form.note }
    if (form.id) {
      await api.put(`/join-groups/${form.id}`, payload)
    } else {
      await api.post('/join-groups', payload)
    }
    showModal.value = false
    await load()
  } catch (e) {
    formError.value = e.response?.data?.error || '保存失败'
  }
}

async function closeGroup(g) {
  if (!confirm(`确认关闭拼合组「${g.code}」？关闭后将不能再增删成员。`)) return
  try {
    await api.post(`/join-groups/${g.id}/close`)
    await load()
  } catch (e) {
    alert(e.response?.data?.error || '关闭失败')
  }
}

async function remove(g) {
  const tip = g.status === 'closed'
    ? `确认删除拼合组「${g.code}」？仅解除拼合关系，不影响文物主数据。`
    : `确认删除拼合组「${g.code}」？组成员关系将一并解除，文物主数据不受影响。`
  if (!confirm(tip)) return
  try {
    await api.delete(`/join-groups/${g.id}`)
    await load()
  } catch (e) {
    alert(e.response?.data?.error || '删除失败')
  }
}

onMounted(load)
</script>

<style scoped>
.filters {
  display: flex;
  gap: 1rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}

.filters label {
  min-width: 220px;
}

.ops {
  white-space: nowrap;
}

.ops .btn {
  margin-right: 0.35rem;
}

.code-link {
  color: var(--accent);
  font-weight: 600;
}

.tag.closed {
  background: #e6e0d6;
  color: var(--muted);
}
</style>
