import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import '@testing-library/jest-dom/vitest'
import Settings from './Settings'
import { I18nProvider } from './i18n'
import type { LLMConfig, ResolvedSetting, SettingsView } from './api'

// F-0414: the ai group page is the panel; a search still lists llm.* keys as
// ordinary registry rows.

const llmKey = (id: string): ResolvedSetting => ({
  id,
  title: id === 'llm.mode' ? 'AI agent' : id,
  type: 'string',
  default: '',
  scopes: ['project', 'host'],
  description: '',
  value: '',
  origin: 'default',
  group: 'ai',
})
const view: SettingsView = {
  files: { project: '/r/.cernerc', host: '/h/settings.toml' },
  settings: ['llm.mode', 'llm.provider', 'llm.model', 'llm.base_url'].map(llmKey),
}
const config: LLMConfig = {
  mode: 'delegation',
  source: 'default',
  clis: [],
  provider: 'anthropic',
  model: '',
  base_url: '',
  key: { env: 'ANTHROPIC_API_KEY', set: false },
}

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  fetchSettings: () => Promise.resolve(view),
  fetchLLMConfig: () => Promise.resolve(config),
}))
vi.mock('./live', () => ({
  useProjects: () => ({ projects: [{ pid: 'p1', name: 'A' }], activePid: 'p1' }),
  useLiveVersion: () => 0,
}))

beforeEach(() => localStorage.setItem('cerne-lang', 'pt-BR'))
afterEach(() => {
  cleanup()
  localStorage.clear()
})

describe('Settings, ai group (F-0414)', () => {
  it('the ai page shows the panel and the Project | Host selector', async () => {
    render(
      <I18nProvider>
        <Settings group="ai" />
      </I18nProvider>,
    )
    expect(await screen.findByText(/Em uso:/)).toBeInTheDocument()
    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { pressed: true }).length + screen.getAllByRole('button', { pressed: false }).length).toBe(2)
  })

  it('a search still lists the llm.* keys as registry rows', async () => {
    render(
      <I18nProvider>
        <Settings group="ai" />
      </I18nProvider>,
    )
    await screen.findByText(/Em uso:/)
    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'llm.mode' } })
    await waitFor(() => expect(screen.queryByRole('radiogroup')).toBeNull())
    expect(screen.getByText('llm.mode')).toBeInTheDocument()
  })
})
