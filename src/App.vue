<template>
  <div class="container">
    <h2 class="title">Управление квартирантами</h2>

    <!-- Фильтр по квартире -->
    <div class="filter">
      <label for="apartmentFilter" class="label">Фильтр по квартире</label>
      <select v-model="selectedApartment" id="apartmentFilter" class="input">
        <option value="">Все квартиры</option>
        <option v-for="apt in uniqueApartments" :key="apt" :value="apt">{{ apt }}</option>
      </select>
    </div>

    <!-- Список квартирантов -->
    <div v-if="filteredTenants.length">
      <div
        v-for="(tenant, index) in filteredTenants"
        :key="index"
        class="tenant-card"
      >
        <div class="tenant-info">
          <strong class="tenant-name">{{ tenant.name }}</strong>
          <p class="tenant-detail">Квартира: {{ tenant.apartment }}</p>
          <p class="tenant-detail">Оплата: {{ formatDate(tenant.paymentDate) }}</p>
        </div>
        <div class="tenant-buttons">
          <button class="btn btn-edit" @click="editTenant(index)">Редактировать</button>
          <button class="btn btn-delete" @click="deleteTenant(index)">Удалить</button>
        </div>
      </div>
    </div>
    <p v-else class="empty">Квартирантов пока нет.</p>

    <!-- Форма для добавления нового квартиранта -->
    <h3 class="subtitle">Добавить нового квартиранта</h3>
    <div class="form">
      <input
        ref="nameInput"
        v-model="newTenant.name"
        placeholder="Имя"
        class="input"
        @keyup.enter="addTenant"
      />
      <input
        ref="apartmentInput"
        v-model="newTenant.apartment"
        placeholder="Квартира"
        class="input"
        @keyup.enter="addTenant"
      />
      <input
        ref="paymentInput"
        v-model="newTenant.paymentDate"
        type="date"
        class="input"
        @keyup.enter="addTenant"
      />
      <button
        class="btn btn-add"
        @click="addTenant"
        :disabled="isLoading"
      >
        {{ isLoading ? 'Отправка...' : 'Добавить' }}
      </button>
    </div>

    <!-- Уведомления -->
    <div v-if="error" class="notification error">{{ error }}</div>
    <div v-if="successMessage" class="notification success">{{ successMessage }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'

const API_URL = 'https://eskertubot.onrender.com/api/tenants'

// Пример данных для демонстрации
const tenants = ref([
  { name: 'Волда Гао', apartment: '1', paymentDate: '2025-05-15' },
  { name: 'Jonseusen', apartment: '2', paymentDate: '2025-02-03' },
  { name: 'Anton Han', apartment: '3', paymentDate: '2025-05-08' },
])

const newTenant = reactive({
  name: '',
  apartment: '',
  paymentDate: ''
})

const selectedApartment = ref('')
const isLoading = ref(false)
const error = ref(null)
const successMessage = ref(null)

// Вычисляем уникальные квартиры для фильтра
const uniqueApartments = computed(() =>
  [...new Set(tenants.value.map(t => t.apartment))]
)

// Фильтрация арендаторов по выбранной квартире
const filteredTenants = computed(() =>
  selectedApartment.value
    ? tenants.value.filter(t => t.apartment === selectedApartment.value)
    : tenants.value
)

async function addTenant() {
  if (!newTenant.name || !newTenant.apartment || !newTenant.paymentDate) {
    error.value = 'Заполните все поля'
    return
  }

  try {
    isLoading.value = true
    error.value = null
    successMessage.value = null

    console.log('Отправляемые данные:', JSON.stringify(newTenant))

    const response = await fetch(API_URL, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(newTenant)
    })

    if (!response.ok) {
      const errorData = await response.json()
      console.error('Ошибка ответа сервера:', errorData)
      throw new Error(errorData.error || 'Ошибка сервера')
    }

    const data = await response.json()
    console.log('Ответ сервера:', data)

    successMessage.value = 'Квартирант добавлен и уведомление отправлено'

    tenants.value.push({ ...newTenant })

    // Очистить форму
    newTenant.name = ''
    newTenant.apartment = ''
    newTenant.paymentDate = ''

    // Скрыть клавиатуру
    blurInputs()
  } catch (err) {
    console.error('Ошибка при отправке:', err)
    error.value = err.message
  } finally {
    isLoading.value = false
  }
}

function deleteTenant(index) {
  tenants.value.splice(index, 1)
}

function editTenant(index) {
  const tenant = tenants.value[index]
  newTenant.name = tenant.name
  newTenant.apartment = tenant.apartment
  newTenant.paymentDate = tenant.paymentDate
  deleteTenant(index)
}

function formatDate(dateStr) {
  const [year, month, day] = dateStr.split('-')
  return `${+day} ${getMonthName(month)}`
}

function getMonthName(month) {
  const months = [
    'января', 'февраля', 'марта', 'апреля', 'мая',
    'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря'
  ]
  return months[+month - 1]
}

// Скрыть клавиатуру
function blurInputs() {
  if ($refs.nameInput) $refs.nameInput.blur()
  if ($refs.apartmentInput) $refs.apartmentInput.blur()
  if ($refs.paymentInput) $refs.paymentInput.blur()
}

<style scoped>
/* Стили для мобильных устройств */
.container {
  background: white;
  max-width: 400px;
  margin: 20px auto;
  border-radius: 20px;
  padding: 20px;
  box-shadow: 0 5px 20px rgba(0, 0, 0, 0.1);
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
}

.title {
  font-size: 1.6rem;
  text-align: center;
  margin-bottom: 15px;
}

.subtitle {
  font-size: 1.3rem;
  margin: 20px 0 10px 0;
  text-align: center;
}

.filter {
  margin-bottom: 15px;
}

.input {
  width: 100%;
  padding: 10px;
  margin-bottom: 15px;
  border-radius: 10px;
  border: 1px solid #ccc;
}

.tenant-card {
  border: 1px solid #ddd;
  border-radius: 15px;
  padding: 15px;
  margin-bottom: 15px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tenant-info {
  line-height: 1.4;
}

.tenant-buttons {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.btn {
  padding: 7px 10px;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.9rem;
}

.btn-edit {
  background-color: #f0f0f0;
  color: #000;
}

.btn-delete {
  background-color: #ff3b30;
  color: white;
}

.btn-add {
  background-color: #007bff;
  color: white;
  font-size: 1rem;
  border-radius: 10px;
  width: 100%;
  padding: 10px;
  margin-top: 10px;
}

.notification {
  margin-top: 10px;
  padding: 8px;
  text-align: center;
  border-radius: 8px;
  font-size: 0.9rem;
}

.notification.error {
  color: #a94442;
  background-color: #f2dede;
}

.notification.success {
  color: #3c763d;
  background-color: #dff0d8;
}

.empty {
  text-align: center;
  color: #666;
  margin-top: 20px;
}

</style>
```

Теперь, когда данные отправляются, клавиатура автоматически убирается.