import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import '@testing-library/jest-dom/vitest'
import Ask from './Ask'
import { I18nProvider } from './i18n'
import type { LLMConfig } from './api'

// F-0414 critério 3: Ask no longer picks the mode; it says which AI answers
// and links to Settings › AI.

const fetchLLMConfig = vi.fn<() => Promise<LLMConfig>>()
vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  fetchLLMConfig: () => fetchLLMConfig(),
}))

const cfg: LLMConfig = {
  mode: 'codex-cli',
  source: 'detected',
  clis: [{ name: 'codex', mode: 'codex-cli', found: true }],
  provider: 'anthropic',
  model: 'gpt-5',
  base_url: '',
  key: { env: 'ANTHROPIC_API_KEY', set: false },
}

beforeEach(() => {
  localStorage.setItem('cerne-lang', 'pt-BR')
  fetchLLMConfig.mockResolvedValue(cfg)
})
afterEach(() => {
  cleanup()
  localStorage.clear()
})

describe('Ask (F-0414)', () => {
  it('critério 3: has no mode selector, shows the AI in use read-only and links to Settings', async () => {
    render(
      <I18nProvider>
        <Ask />
      </I18nProvider>,
    )
    expect(await screen.findByText('Assinatura — Codex CLI · gpt-5')).toBeInTheDocument()
    expect(screen.queryByRole('combobox')).toBeNull()
    expect(screen.getByRole('link', { name: 'Configurar' })).toHaveAttribute('href', '#/settings/ai')
  })

  it('item 11: a CLI with no model says "padrão do CLI", and the intro links to nothing browser-blue', async () => {
    fetchLLMConfig.mockResolvedValue({ ...cfg, model: '' })
    render(
      <I18nProvider>
        <Ask />
      </I18nProvider>,
    )
    expect(await screen.findByText('Assinatura — Codex CLI · padrão do CLI')).toBeInTheDocument()
  })
})
