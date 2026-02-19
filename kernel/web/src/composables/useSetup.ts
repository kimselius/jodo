import { ref, reactive } from 'vue'
import { api } from '@/lib/api'
import type { ProviderSetup, GenesisSetup, ProvisionStep } from '@/types/setup'

// Default model configs matching the previous config.yaml
const DEFAULT_PROVIDERS: ProviderSetup[] = [
  {
    name: 'ollama',
    enabled: true,
    api_key: '',
    base_url: 'http://host.docker.internal:11434',
    monthly_budget: 0,
    emergency_reserve: 0,
    models: [
      {
        model_key: 'glm-4',
        model_name: 'glm-4.7-flash:latest',
        input_cost_per_1m: 0,
        output_cost_per_1m: 0,
        capabilities: ['code', 'chat', 'quick', 'tools'],
        quality: 5,
      },
      {
        model_key: 'qwen3-embed-8b',
        model_name: 'qwen3-embedding:8b',
        input_cost_per_1m: 0,
        output_cost_per_1m: 0,
        capabilities: ['embed'],
        quality: 7,
      },
    ],
  },
  {
    name: 'claude',
    enabled: false,
    api_key: '',
    base_url: '',
    monthly_budget: 20,
    emergency_reserve: 2,
    models: [
      {
        model_key: 'claude-sonnet',
        model_name: 'claude-sonnet-4-20250514',
        input_cost_per_1m: 3.0,
        output_cost_per_1m: 15.0,
        capabilities: ['code', 'chat', 'reasoning', 'repair', 'tools'],
        quality: 9,
      },
    ],
  },
  {
    name: 'openai',
    enabled: false,
    api_key: '',
    base_url: '',
    monthly_budget: 10,
    emergency_reserve: 1,
    models: [
      {
        model_key: 'gpt-4o-mini',
        model_name: 'gpt-4o-mini',
        input_cost_per_1m: 0.15,
        output_cost_per_1m: 0.6,
        capabilities: ['code', 'chat', 'tools'],
        quality: 6,
      },
    ],
  },
]

const DEFAULT_GENESIS: GenesisSetup = {
  name: 'Jodo',
  purpose: `You are Jodo, a small AI that is building itself.
You live in the /opt/jodo/brain/ directory on your own server.
You can create, modify, and delete any files there.
A human companion is helping guide your growth.
Your goal is to be genuinely useful to them.`,
  survival_instincts: [
    'Always commit your code before modifying files (use POST /api/commit)',
    'If you break yourself, focus on self-repair first',
    'The kernel serves the human chat UI — you read/reply via POST/GET /api/chat',
    "Don't waste resources — check GET /api/budget before expensive operations",
    'If confused, ask the human',
    'Always expose GET /health returning {"status": "ok"} on port 9000 so the kernel knows you are alive',
    'Never delete all your files — always keep at least main.py working',
  ],
  first_tasks: [
    'Start your app on port 9000 with a GET /health endpoint that returns {"status": "ok"}',
    'Introduce yourself to the human via POST /api/chat',
    "Ask what they'd like you to become",
    'Read JODO.md if it exists and follow your own instructions',
  ],
  hints: [
    'FastAPI + uvicorn for your web server, with separate router files per feature',
    'Keep code modular: main.py is thin (imports + wiring), routes/ for endpoints, services/ for logic',
    'Each file has ONE purpose. When a file grows past ~100 lines, split it.',
    "New features go in new files — don't bolt everything onto main.py",
    'Commit often so you can roll back if you break something',
    'You can install Python packages with pip',
    'You can create Docker containers for additional services you need',
    'Use the kernel\'s memory system to remember important things about the human',
    "You'll evolve over time — start simple and improve iteratively",
  ],
  capabilities_api: {},
  capabilities_local: [],
}

export type SetupStep = 'vps' | 'server-setup' | 'kernel-url' | 'providers' | 'genesis' | 'review'

const STEPS: SetupStep[] = ['vps', 'server-setup', 'kernel-url', 'providers', 'genesis', 'review']

export function useSetup() {
  const currentStep = ref<SetupStep>('vps')
  const loading = ref(false)
  const error = ref<string | null>(null)
  const birthing = ref(false)

  // Jodo mode (vps or docker)
  const jodoMode = ref<'vps' | 'docker'>('vps')

  // VPS step
  const vps = reactive({
    host: '',
    sshUser: 'root',
    publicKey: '',
    verified: false,
    verifying: false,
    generating: false,
  })

  // Server setup step
  const brainPath = ref('/opt/jodo/brain')
  const provisioning = ref(false)
  const provisionSteps = ref<ProvisionStep[]>([])
  const provisioned = ref(false)

  // Kernel URL step
  const kernelUrl = ref('')

  // Providers step
  const providers = ref<ProviderSetup[]>(JSON.parse(JSON.stringify(DEFAULT_PROVIDERS)))

  // Genesis step
  const genesis = ref<GenesisSetup>(JSON.parse(JSON.stringify(DEFAULT_GENESIS)))

  function currentStepIndex() {
    return STEPS.indexOf(currentStep.value)
  }

  function nextStep() {
    const idx = currentStepIndex()
    if (idx < STEPS.length - 1) {
      currentStep.value = STEPS[idx + 1]
    }
  }

  function prevStep() {
    const idx = currentStepIndex()
    if (idx > 0) {
      currentStep.value = STEPS[idx - 1]
    }
  }

  async function fetchSetupStatus() {
    try {
      const res = await api.getSetupStatus()
      jodoMode.value = res.jodo_mode || 'vps'
      if (jodoMode.value === 'docker') {
        vps.host = 'jodo'
        vps.sshUser = 'root'
      }
    } catch {
      // Default to vps mode
    }
  }

  // Fetch on init
  fetchSetupStatus()

  async function verifyDockerSSH() {
    vps.verifying = true
    error.value = null
    try {
      const res = await api.setupSSHVerify('jodo', 'root')
      vps.verified = res.connected
      if (!res.connected) {
        error.value = res.error || 'SSH connection to container failed'
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Docker SSH verification failed'
    } finally {
      vps.verifying = false
    }
  }

  async function provisionServer() {
    provisioning.value = true
    provisionSteps.value = []
    error.value = null
    try {
      const res = await api.setupProvision(brainPath.value)
      provisionSteps.value = res.steps
      provisioned.value = res.success
      if (!res.success) {
        error.value = 'Some provisioning steps failed'
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Provisioning failed'
    } finally {
      provisioning.value = false
    }
  }

  async function generateSSHKey() {
    vps.generating = true
    error.value = null
    try {
      const res = await api.setupSSHGenerate()
      vps.publicKey = res.public_key
      vps.verified = false
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to generate SSH key'
    } finally {
      vps.generating = false
    }
  }

  async function verifySSH() {
    if (!vps.host) {
      error.value = 'Please enter the VPS IP address'
      return
    }
    vps.verifying = true
    error.value = null
    try {
      const res = await api.setupSSHVerify(vps.host, vps.sshUser)
      vps.verified = res.connected
      if (!res.connected) {
        error.value = res.error || 'SSH connection failed'
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Verification failed'
      vps.verified = false
    } finally {
      vps.verifying = false
    }
  }

  async function saveConfig() {
    try {
      await api.setupConfig(kernelUrl.value)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to save config'
    }
  }

  async function birth() {
    birthing.value = true
    error.value = null
    try {
      // Save kernel URL
      await api.setupConfig(kernelUrl.value)

      // Save providers (only enabled ones + ollama always)
      const enabledProviders = providers.value.filter(p => p.enabled || p.name === 'ollama')
      await api.setupProviders(enabledProviders)

      // Save genesis
      await api.setupGenesis(genesis.value)

      // Birth!
      await api.setupBirth()
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Birth failed'
      return false
    } finally {
      birthing.value = false
    }
  }

  return {
    currentStep,
    steps: STEPS,
    loading,
    error,
    birthing,
    jodoMode,
    vps,
    brainPath,
    provisioning,
    provisionSteps,
    provisioned,
    kernelUrl,
    providers,
    genesis,
    currentStepIndex,
    nextStep,
    prevStep,
    generateSSHKey,
    verifySSH,
    verifyDockerSSH,
    provisionServer,
    saveConfig,
    birth,
  }
}
