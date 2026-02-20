import { chatApi } from './chat'
import { statusApi } from './status'
import { genesisApi } from './genesis'
import { llmApi } from './llm'
import { memoryApi } from './memory'
import { growthApi } from './growth'
import { lifecycleApi } from './lifecycle'
import { setupApi } from './setup'
import { settingsApi } from './settings'
import { libraryApi } from './library'

export const api = {
  // Chat
  getMessages: chatApi.getMessages,
  sendMessage: chatApi.sendMessage,
  ackMessages: chatApi.ackMessages,

  // Status
  getStatus: statusApi.getStatus,
  getBudget: statusApi.getBudget,
  getBudgetBreakdown: statusApi.getBudgetBreakdown,

  // Genesis
  getGenesis: genesisApi.getGenesis,
  updateIdentity: genesisApi.updateIdentity,

  // LLM Calls
  getLLMCalls: llmApi.getLLMCalls,
  getLLMCallDetail: llmApi.getLLMCallDetail,

  // Memory
  getMemories: memoryApi.getMemories,
  searchMemories: memoryApi.searchMemories,

  // Growth
  getGrowth: growthApi.getGrowth,
  getGallas: growthApi.getGallas,

  // Lifecycle
  restart: lifecycleApi.restart,

  // Setup
  getSetupStatus: setupApi.getSetupStatus,
  setupSSHGenerate: setupApi.setupSSHGenerate,
  setupSSHVerify: setupApi.setupSSHVerify,
  setupConfig: setupApi.setupConfig,
  setupProviders: setupApi.setupProviders,
  setupGenesis: setupApi.setupGenesis,
  setupTestProvider: setupApi.setupTestProvider,
  setupBirth: setupApi.setupBirth,
  setupProvision: setupApi.setupProvision,
  setupRouting: setupApi.setupRouting,
  setupDiscoverModels: setupApi.setupDiscoverModels,

  // Settings
  getSettingsProviders: settingsApi.getProviders,
  updateProvider: settingsApi.updateProvider,
  addModel: settingsApi.addModel,
  updateModel: settingsApi.updateModel,
  deleteModel: settingsApi.deleteModel,
  discoverModels: settingsApi.discoverModels,
  getSettingsGenesis: settingsApi.getGenesis,
  updateSettingsGenesis: settingsApi.updateGenesis,
  getSettingsRouting: settingsApi.getRouting,
  updateSettingsRouting: settingsApi.updateRouting,
  getSettingsKernel: settingsApi.getKernel,
  updateSettingsKernel: settingsApi.updateKernel,
  getSettingsSSH: settingsApi.getSSH,
  getSettingsSubagent: settingsApi.getSubagent,
  updateSettingsSubagent: settingsApi.updateSubagent,
  getVRAMStatus: settingsApi.getVRAMStatus,

  // Library & Inbox
  getLibrary: libraryApi.getLibrary,
  createLibraryItem: libraryApi.createItem,
  updateLibraryItem: libraryApi.updateItem,
  patchLibraryStatus: libraryApi.patchStatus,
  deleteLibraryItem: libraryApi.deleteItem,
  addLibraryComment: libraryApi.addComment,
  getInbox: libraryApi.getInbox,
}
