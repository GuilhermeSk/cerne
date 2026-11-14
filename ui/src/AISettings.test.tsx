import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import '@testing-library/jest-dom/vitest'
import AISettings from './AISettings'
import { I18nProvider } from './i18n'
import type { LLMConfig, LLMKeyStatus, ResolvedSetting, SettingScope, SettingsView } from './api'

// F-0414: the AI group's panel. It knows no endpoint shape beyond the api
// module; every write goes through putSetting/unsetSetting (llm.* keys) or the
// key endpoints, in the scope the page selector shows.

const fetchLLMConfig = vi.fn<() => Promise<LLMConfig>>()
const testLLM = vi.fn<(mode?: string) => Promise<{ ok: boolean; error?: string }>>()
const putLLMKey = vi.fn<(provider: string, value: string) => Promise<LLMKeyStatus>>()
const deleteLLMKey = vi.fn<(provider: string) => Promise<LLMKeyStatus>>()
const putSetting = vi.fn<(id: string, scope: SettingScope, value: unknown) => Promise<SettingsView>>()
const unsetSetting = vi.fn<(id: string, scope: SettingScope) => Promise<SettingsView>>()

vi.mock('./api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./api')>()),
  fetchLLMConfig: () => fetchLLMConfig(),
  testLLM: (m?: string) => testLLM(m),
  putLLMKey: (p: string, v: string) => putLLMKey(p, v),
  deleteLLMKey: (p: string) => deleteLLMKey(p),
  putSetting: (id: string, scope: SettingScope, value: unknown) => putSetting(id, scope, value),
  unsetSetting: (id: string, scope: SettingScope) => unsetSetting(id, scope),
}))

const view: SettingsView = { settings: [], files: {} }
const reg = (id: string, over: Partial<ResolvedSetting> = {}): ResolvedSetting => ({
  id,
  title: id,
  type: 'string',
  default: '',
  scopes: ['project', 'host'],
  description: '',
  value: '',
  origin: 'default',
  group: 'ai',
  ...over,
})
const noStored = [reg('llm.mode'), reg('llm.provider'), reg('llm.model'), reg('llm.base_url')]

const cfg = (over: Partial<LLMConfig> = {}): LLMConfig => ({
  mode: 'claude-cli',
  source: 'detected',
  clis: [
    { name: 'claude', mode: 'claude-cli', found: true },
    { name: 'codex', mode: 'codex-cli', found: false },
    { name: 'gemini', mode: 'gemini-cli', found: true },
  ],
  provider: 'anthropic',
  model: '',
  base_url: '',
  key: { env: 'ANTHROPIC_API_KEY', set: false },
  models: [
    { id: 'claude-opus-4-8', label: 'Opus 4.8', provider: 'anthropic', visibility: 'current' },
    { id: 'claude-old', label: 'Old', provider: 'anthropic', visibility: 'delisted' },
  ],
  default_model: 'claude-opus-4-8',
  ...over,
})

function mount(settings = noStored, scope: SettingScope = 'project') {
  const onWritten = vi.fn()
  render(
    <I18nProvider>
      <AISettings settings={settings} scope={scope} onWritten={onWritten} />
    </I18nProvider>,
  )
  return onWritten
}

beforeEach(() => {
  localStorage.setItem('cerne-lang', 'pt-BR')
  fetchLLMConfig.mockResolvedValue(cfg())
  putSetting.mockResolvedValue(view)
  unsetSetting.mockResolvedValue(view)
  testLLM.mockResolvedValue({ ok: true })
})
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  localStorage.clear()
})

const pick = (name: string) => fireEvent.click(screen.getByRole('radio', { name }))

describe('AISettings (F-0414)', () => {
  it('critério 1: sem valor gravado, diz que a CLI foi detectada no PATH', async () => {
    mount()
    const line = await screen.findByText(/Em uso:/)
    expect(line.parentElement).toHaveTextContent('Assinatura — Claude Code')
    expect(line.parentElement).toHaveTextContent('detectado no PATH')
  })

  it('critério 1: valor gravado diz em que escopo foi configurado', async () => {
    fetchLLMConfig.mockResolvedValue(cfg({ source: 'configured', mode: 'gemini-cli' }))
    mount([reg('llm.mode', { stored: { host: 'gemini-cli' } }), ...noStored.slice(1)])
    const line = await screen.findByText(/Em uso:/)
    expect(line.parentElement).toHaveTextContent('Gemini CLI')
    expect(line.parentElement).toHaveTextContent('configurado no Host')
  })

  it('critério 2: os quatro caminhos são um grupo de rádio rotulado e a Assinatura lista as três CLIs', async () => {
    mount()
    const group = await screen.findByRole('radiogroup', { name: /Como o cerne chega a um modelo/ })
    expect(within(group).getAllByRole('radio').map((r) => (r as HTMLInputElement).value)).toEqual([
      'subscription',
      'api',
      'gateway',
      'delegation',
    ])
    expect(screen.getByRole('radio', { name: /Assinatura/ })).toBeChecked()
    expect(screen.getByText('Claude Code').closest('li')).toHaveTextContent('instalado')
    expect(screen.getByText('Codex CLI').closest('li')).toHaveTextContent('não encontrado')
  })

  it('critério 2: Usar numa CLI grava llm.mode no escopo selecionado', async () => {
    mount(noStored, 'host')
    fireEvent.click(await screen.findByRole('button', { name: 'Usar Gemini CLI' }))
    await waitFor(() => expect(putSetting).toHaveBeenCalledWith('llm.mode', 'host', 'gemini-cli'))
  })

  it('critério 2: Usar numa CLI não instalada não grava nada', async () => {
    mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Usar Codex CLI' }))
    expect(putSetting).not.toHaveBeenCalled()
  })

  it('critério 2: Delegação explica o caminho e Usar grava llm.mode=delegation', async () => {
    mount()
    await screen.findByText(/Em uso:/)
    pick('Delegação')
    expect(screen.getAllByText(/O agente da sessão em curso responde/).length).toBeGreaterThan(0)
    fireEvent.click(screen.getByRole('button', { name: 'Usar Delegação' }))
    await waitFor(() => expect(putSetting).toHaveBeenCalledWith('llm.mode', 'project', 'delegation'))
  })

  it('critério 4: Testar mostra OK numa região viva educada', async () => {
    mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Testar Claude Code' }))
    const live = await screen.findByText('Claude Code: conexão OK.')
    expect(live.closest('[aria-live="polite"]')).not.toBeNull()
    expect(testLLM).toHaveBeenCalledWith('claude-cli')
  })

  it('critério 4: Testar com erro mostra o erro com a correção que o servidor nomeou', async () => {
    testLLM.mockResolvedValue({ ok: false, error: 'not signed in: run `gemini` and sign in' })
    mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Testar Gemini CLI' }))
    const msg = await screen.findByText(/Gemini CLI: falhou\. not signed in: run `gemini` and sign in/)
    expect(msg.closest('[aria-live="polite"]')).not.toBeNull()
  })

  it('critério 7: Chave de API grava mode, provider e modelo; o modelo da Anthropic é um select do catálogo sem os deslistados', async () => {
    mount()
    await screen.findByText(/Em uso:/)
    pick('Chave de API')
    const model = screen.getByRole('combobox', { name: 'Modelo' })
    expect(within(model).getAllByRole('option').map((o) => o.textContent)).toEqual(['Padrão do catálogo', 'Opus 4.8'])
    fireEvent.change(model, { target: { value: 'claude-opus-4-8' } })
    fireEvent.click(screen.getByRole('button', { name: 'Usar Chave de API' }))
    await waitFor(() => expect(putSetting).toHaveBeenCalledWith('llm.model', 'project', 'claude-opus-4-8'))
    expect(putSetting).toHaveBeenCalledWith('llm.mode', 'project', 'byo-key')
    expect(putSetting).toHaveBeenCalledWith('llm.provider', 'project', 'anthropic')
    expect(unsetSetting).toHaveBeenCalledWith('llm.base_url', 'project')
  })

  it('critério 7: fora da Anthropic o modelo é texto livre', async () => {
    mount()
    await screen.findByText(/Em uso:/)
    pick('Chave de API')
    fireEvent.change(screen.getByRole('combobox', { name: 'Provedor' }), { target: { value: 'openai' } })
    const model = screen.getByRole('textbox', { name: 'Modelo' })
    fireEvent.change(model, { target: { value: 'gpt-5' } })
    fireEvent.click(screen.getByRole('button', { name: 'Usar Chave de API' }))
    await waitFor(() => expect(putSetting).toHaveBeenCalledWith('llm.model', 'project', 'gpt-5'))
    expect(putSetting).toHaveBeenCalledWith('llm.provider', 'project', 'openai')
  })

  it('critério 8: compatível pede a URL base e a grava; modelo vazio remove a chave do escopo', async () => {
    mount(noStored, 'host')
    await screen.findByText(/Em uso:/)
    pick('Gateway ou compatível')
    fireEvent.change(screen.getByRole('combobox', { name: 'Gateway' }), { target: { value: 'compatible' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'URL base' }), { target: { value: 'http://localhost:11434/v1' } })
    fireEvent.click(screen.getByRole('button', { name: 'Usar Gateway ou compatível' }))
    await waitFor(() => expect(putSetting).toHaveBeenCalledWith('llm.base_url', 'host', 'http://localhost:11434/v1'))
    expect(putSetting).toHaveBeenCalledWith('llm.provider', 'host', 'compatible')
    expect(putSetting).toHaveBeenCalledWith('llm.mode', 'host', 'byo-key')
    expect(unsetSetting).toHaveBeenCalledWith('llm.model', 'host')
  })

  it('critério 8: a chave salva nunca volta ao DOM, só os 4 últimos, e o campo limpa', async () => {
    putLLMKey.mockResolvedValue({ env: 'ANTHROPIC_API_KEY', set: true, source: 'file', last4: 'wxyz' })
    const { container } = { container: document.body }
    mount()
    await screen.findByText(/Em uso:/)
    pick('Chave de API')
    const field = screen.getByLabelText('Chave', { selector: 'input' }) as HTMLInputElement
    expect(field.type).toBe('password')
    fireEvent.change(field, { target: { value: 'sk-secret-1234567890wxyz' } })
    fireEvent.click(screen.getByRole('button', { name: 'Salvar chave' }))
    await screen.findByText('•••• wxyz')
    expect(putLLMKey).toHaveBeenCalledWith('anthropic', 'sk-secret-1234567890wxyz')
    expect(field.value).toBe('')
    expect(container.innerHTML).not.toContain('sk-secret')
  })

  it('critério 8: chave vinda do ambiente diz a variável e não oferece Remover', async () => {
    fetchLLMConfig.mockResolvedValue(cfg({ key: { env: 'ANTHROPIC_API_KEY', set: true, source: 'env' } }))
    mount()
    await screen.findByText(/Em uso:/)
    pick('Chave de API')
    expect(screen.getByText('definida pela variável ANTHROPIC_API_KEY')).toBeInTheDocument()
    expect(screen.getByText(/só neste computador/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Remover chave' })).toBeNull()
  })

  it('critério 8: Remover chave chama o DELETE e volta a "nenhuma chave"', async () => {
    fetchLLMConfig.mockResolvedValue(cfg({ key: { env: 'ANTHROPIC_API_KEY', set: true, source: 'file', last4: 'wxyz' } }))
    deleteLLMKey.mockResolvedValue({ env: 'ANTHROPIC_API_KEY', set: false })
    mount()
    await screen.findByText(/Em uso:/)
    pick('Chave de API')
    expect(screen.getByText('•••• wxyz')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Remover chave' }))
    await waitFor(() => expect(deleteLLMKey).toHaveBeenCalledWith('anthropic'))
    await screen.findByText('Nenhuma chave definida.')
  })

  it('critério 11: uma escrita que falha avisa o motivo em um alerta', async () => {
    putSetting.mockRejectedValue(new Error('the lock is busy'))
    mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Usar Gemini CLI' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Não gravou: the lock is busy')
  })

  it('uma escrita que passa entrega a view nova a quem hospeda o painel', async () => {
    const onWritten = mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Usar Gemini CLI' }))
    await waitFor(() => expect(onWritten).toHaveBeenCalledWith(view))
  })
})
