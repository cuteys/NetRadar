<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue'
import { useAuthStore } from './stores/authStore'
import { useRadarStore } from './stores/radarStore'
import { useThemeStore } from './stores/themeStore'
import HeaderBar from './components/HeaderBar.vue'
import CumulativeStatsBar from './components/CumulativeStatsBar.vue'
import LoginView from './components/LoginView.vue'
import MapFlylines from './components/MapFlylines.vue'
import ThroughputWave from './components/ThroughputWave.vue'
import DeviceRanking from './components/DeviceRanking.vue'
import ProtocolDonut from './components/ProtocolDonut.vue'
import LiveLogStream from './components/LiveLogStream.vue'

const auth = useAuthStore()
const radar = useRadarStore()
const theme = useThemeStore()

onMounted(async () => {
  await auth.checkAuth()
  if (auth.isAuthenticated) {
    radar.fetchNodes()
    radar.fetchSystemSettings()
    radar.fetchCumulativeStats()
    radar.connect()
  }
})

watch(() => auth.isAuthenticated, (val) => {
  if (val) {
    radar.fetchNodes()
    radar.fetchSystemSettings()
    radar.fetchCumulativeStats()
    radar.connect()
  } else {
    radar.disconnect()
  }
})

onUnmounted(() => {
  radar.disconnect()
})
</script>

<template>
  <!-- If not logged in, display Apple-style Login View -->
  <LoginView v-if="!auth.isAuthenticated" />

  <!-- Main Dashboard View -->
  <div v-else class="min-h-screen p-3 sm:p-5 max-w-[1600px] mx-auto flex flex-col transition-colors duration-300">
    <!-- Top Navigation & Rate Badges -->
    <HeaderBar />

    <!-- Cumulative Metrics & Time Range Selector -->
    <CumulativeStatsBar />

    <!-- Main Content Layout -->
    <main class="flex-1 flex flex-col gap-4 sm:gap-5">
      <!-- 1. Central Situational Map with Flying Particles -->
      <section>
        <MapFlylines />
      </section>

      <!-- 2. Three Analytics Cards Grid (Mobile Responsive 1-col -> 2-col -> 3-col) -->
      <section class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-5">
        <ThroughputWave />
        <DeviceRanking />
        <ProtocolDonut />
      </section>

      <!-- 3. Real-time Live Outbound Log Stream -->
      <section>
        <LiveLogStream />
      </section>
    </main>

    <!-- Footer -->
    <footer class="mt-6 py-3 text-center text-xs text-slate-400 dark:text-slate-500 border-t border-slate-200/50 dark:border-slate-800/50">
      Designed by ChuiYan 💖
    </footer>
  </div>
</template>
