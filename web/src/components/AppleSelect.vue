<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ChevronDown, Check } from 'lucide-vue-next'

export interface DropdownOption {
  label: string
  value: string | number
  badge?: string
  badgeColor?: 'emerald' | 'slate' | 'sky' | 'amber'
  sublabel?: string
}

const props = withDefaults(
  defineProps<{
    modelValue: string | number
    options: DropdownOption[]
    placeholder?: string
    menuWidthClass?: string
    buttonClass?: string
    placement?: 'left' | 'right'
  }>(),
  {
    placeholder: '请选择',
    menuWidthClass: 'min-w-[190px]',
    buttonClass: '',
    placement: 'left',
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', val: string | number): void
  (e: 'change', val: string | number): void
}>()

const isOpen = ref(false)
const dropdownRef = ref<HTMLDivElement | null>(null)

const selectedOption = computed(() => {
  return props.options.find((o) => o.value === props.modelValue)
})

const toggleDropdown = () => {
  isOpen.value = !isOpen.value
}

const selectOption = (opt: DropdownOption) => {
  emit('update:modelValue', opt.value)
  emit('change', opt.value)
  isOpen.value = false
}

const handleKeyDown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && isOpen.value) {
    isOpen.value = false
  }
}

const handleClickOutside = (e: MouseEvent) => {
  if (dropdownRef.value && !dropdownRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('pointerdown', handleClickOutside)
  document.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  document.removeEventListener('pointerdown', handleClickOutside)
  document.removeEventListener('keydown', handleKeyDown)
})
</script>

<template>
  <div ref="dropdownRef" class="relative inline-block text-left select-none">
    <!-- Trigger Button -->
    <button
      type="button"
      @click="toggleDropdown"
      class="group inline-flex items-center justify-between gap-1.5 transition-all outline-none focus:outline-none"
      :class="buttonClass"
      :aria-expanded="isOpen"
    >
      <slot name="prefix"></slot>

      <span class="truncate font-medium">
        {{ selectedOption?.label || placeholder }}
      </span>

      <slot name="badge">
        <span
          v-if="selectedOption?.badge"
          class="text-[10px] px-1.5 py-0.2 rounded-md font-mono flex-shrink-0"
          :class="selectedOption.badgeColor === 'emerald'
            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
            : 'bg-slate-500/10 text-slate-500 dark:text-slate-400'"
        >
          {{ selectedOption.badge }}
        </span>
      </slot>

      <ChevronDown
        class="w-3.5 h-3.5 text-slate-400 dark:text-slate-400 transition-transform duration-200 flex-shrink-0"
        :class="{ 'rotate-180': isOpen }"
      />
    </button>

    <!-- Floating Dropdown Menu -->
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="transform scale-95 opacity-0 -translate-y-1"
      enter-to-class="transform scale-100 opacity-100 translate-y-0"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="transform scale-100 opacity-100 translate-y-0"
      leave-to-class="transform scale-95 opacity-0 -translate-y-1"
    >
      <div
        v-if="isOpen"
        class="absolute mt-1.5 z-50 rounded-2xl apple-glass-heavy shadow-xl border border-slate-200/80 dark:border-slate-700/80 p-1.5 backdrop-blur-xl max-h-64 overflow-y-auto"
        :class="[menuWidthClass, placement === 'right' ? 'right-0' : 'left-0']"
      >
        <div class="space-y-0.5">
          <button
            v-for="opt in options"
            :key="opt.value"
            type="button"
            @click="selectOption(opt)"
            class="w-full text-left px-2.5 py-1.5 rounded-xl text-xs flex items-center justify-between gap-2 transition-colors group cursor-pointer"
            :class="opt.value === modelValue
              ? 'bg-emerald-50/80 dark:bg-emerald-950/40 text-emerald-700 dark:text-emerald-300 font-semibold'
              : 'text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800/80 font-normal'"
          >
            <div class="flex items-center gap-1.5 min-w-0 flex-1 truncate">
              <span class="truncate">{{ opt.label }}</span>
              <span
                v-if="opt.badge"
                class="text-[10px] px-1.5 py-0.2 rounded-md font-mono flex-shrink-0"
                :class="opt.badgeColor === 'emerald'
                  ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400'
                  : 'bg-slate-500/15 text-slate-500 dark:text-slate-400'"
              >
                {{ opt.badge }}
              </span>
              <span
                v-if="opt.sublabel"
                class="text-[10px] text-slate-400 font-mono truncate"
              >
                ({{ opt.sublabel }})
              </span>
            </div>

            <Check
              v-if="opt.value === modelValue"
              class="w-3.5 h-3.5 text-emerald-500 flex-shrink-0 ml-1.5"
            />
          </button>
        </div>
      </div>
    </Transition>
  </div>
</template>
