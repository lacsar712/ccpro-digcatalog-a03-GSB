<template>
  <div>
    <div class="toolbar">
      <div>
        <h2 class="page-title">残片拼合组</h2>
        <p class="page-sub">将同一器物的残片文物归为拼合组，跟踪拼对与修复进度</p>
      </div>
      <button class="btn" @click="openCreate">新建拼合组</button>
    </div>

    <div class="card">
      <div class="filters">
        <label>
          状态筛选
          <select v-model="filterStatus" @change="load">
            <option value="">全部状态</option>
            <option value="open">进行中</option>
            <option value="closed">已关闭</option>
          </select>
        </label>
      </div>

      <table class="table">
        <thead>
          <tr>
            <th>编号</th>
            <th>名称</th>
            <th>状态</th>
            <th>成员数</th>
            <th>备注</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in list" :key="item.id">
            <td>
              <router-link :to="`/join-groups/${item.id}`">{{ item.code }}</router-link>
            </td>
            <td>{{ item.title }}</td>
            <td>
              <span class="tag" :class="item.status === 'open' ? 'ok' : 'closed'">
                {{ item.status === 'open' ? '进行中' : '已关闭' }}
              </span>
            </td>
            <td>{{ item.memberCount ?? 0 }}</td>
            <td>{{ item.note || '-' }}</td>
            <td>{{ formatDate(item.createdAt) }}</td>
            <td class="actions">
              <router-link class="btn secondary small" :to="`/join-groups/${item.id}`">详情</router-link>
              <button class="btn secondary small" @click="openEdit(item)">编辑</button>
              <button v-if="item.status === 'open'" class="btn small" @click="close(item)">关闭</button>
              <button class="btn danger small" @click="remove(item)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-if="!list.length" class="page-sub">暂无数据</p>
      <p v-if="error" class="error">{{ error }}</p>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="showModal = false">
      <div class="modal">
        <h3>{{ form.id ? '编辑拼合组' : '新建拼合组' }}</h3>
        <div class="form-grid">
          <label>
            编号（全库唯一）
            <input v-model="form.code" placeholder="如 JG-2024-004" />
          </label>
          <label>
            名称
            <input v-model="form.title" placeholder="如 灰陶罐残片拼合" />
          </label>
          <label class="full">
            备注
            <textarea v-model="form.note" />
          </label>
        </div>
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

const form = reactive({
  id: null,
  code: '',
  title: '',
  note: ''
})

function formatDate(v) {
  if (!v) return '-'
  return String(v).slice(0, 10)
}

async function load() {
  error.value = ''
  try {
    const params = {}
    if (filterStatus.value) params.status = filterStatus.value
    const { data } = await api.get('/joingroups', { params })
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

function openEdit(item) {
  Object.assign(form, { id: item.id, code: item.code, title: item.title, note: item.note || '' })
  formError.value = ''
  showModal.value = true
}

async function save() {
  formError.value = ''
  try {
    const payload = { code: form.code, title: form.title, note: form.note }
    if (form.id) {
      await api.put(`/joingroups/${form.id}`, payload)
    } else {
      await api.post('/joingroups', payload)
    }
    showModal.value = false
    await load()
  } catch (e) {
    formError.value = e.response?.data?.error || '保存失败'
  }
}

async function close(item) {
  if (!confirm(`确认关闭拼合组「${item.code}」？关闭后禁止再增删成员。`)) return
  try {
    await api.post(`/joingroups/${item.id}/close`)
    await load()
  } catch (e) {
    alert(e.response?.data?.error || '关闭失败')
  }
}

async function remove(item) {
  if (!confirm(`确认删除拼合组「${item.code}」？组内成员关联将一并移除。`)) return
  try {
    await api.delete(`/joingroups/${item.id}`)
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
  min-width: 200px;
}

.actions {
  white-space: nowrap;
}

.actions .btn {
  margin-right: 0.4rem;
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
