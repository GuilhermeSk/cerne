import { useEffect, useId, useState } from 'react'
import {
  deleteLLMKey,
  fetchLLMConfig,
  putLLMKey,
  putSetting,
  testLLM,
  unsetSetting,
  type LLMConfig,
  type LLMKeyStatus,
  type LLMMode,
  type ResolvedSetting,
  type SettingScope,
  type SettingsView,
} from './api'
import { useT, type Messages } from './i18n'

// The AI group's panel (F-0414): which of the four access paths cerne reaches
// a model by, written to the llm.* registry keys in the scope the Settings page
// selector shows. The API key never travels here except on its way to the host
// credentials file; the page only ever learns its last four characters.

type Texts = Messages['ai']
type PathId = 'subscription' | 'api' | 'gateway' | 'delegation'
const PATHS: PathId[] = ['subscription', 'api', 'gateway', 'delegation']
const PROVIDERS: Record<'api' | 'gateway', string[]> = {
  api: ['anthropic', 'openai', 'gemini'],
  gateway: ['openrouter', 'compatible'],
}

function pathOf(c: LLMConfig): PathId {
  if (c.mode === 'delegation') return 'delegation'
  if (c.mode === 'byo-key') return PROVIDERS.gateway.includes(c.provider) ? 'gateway' : 'api'
  return 'subscription'
}

// describeLLM is the one-line answer to "which AI is in use": path, the CLI or
// provider behind it, and the model. The Ask screen shows the same line.
export function describeLLM(c: LLMConfig, ai: Texts): string {
  const path = pathOf(c)
  const title = ai.paths[path].title
  if (path === 'delegation') return `${title} — ${ai.delegationWho}`
  const cli = c.clis.find((x) => x.mode === c.mode)
  const who = path === 'subscription' ? (ai.clis[cli?.name ?? ''] ?? c.mode) : (ai.providers[c.provider] ?? c.provider)
  const catalog = c.provider === 'anthropic' ? c.models?.find((m) => m.id === c.default_model)?.label : undefined
  const model = c.model || (path !== 'subscription' ? catalog : undefined) || ai.modelDefault
  return `${title} — ${who} · ${model}`
}

export default function AISettings({
  settings,
  scope,
  onWritten,
}: {
  settings: ResolvedSetting[]
  scope: SettingScope
  onWritten: (view: SettingsView) => void
}) {
  const { t } = useT()
  const ai = t.ai
  const uid = useId()
  const [cfg, setCfg] = useState<LLMConfig | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [path, setPath] = useState<PathId | null>(null)
  const [provider, setProvider] = useState('')
  const [model, setModel] = useState<string | null>(null)
  const [baseURL, setBaseURL] = useState<string | null>(null)
  const [keyInput, setKeyInput] = useState('')
  const [keys, setKeys] = useState<Readonly<Record<string, LLMKeyStatus>>>({})
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [writeError, setWriteError] = useState<string | null>(null)

  const load = () => {
    setLoadError(null)
    fetchLLMConfig().then(setCfg, (e: unknown) => setLoadError(e instanceof Error ? e.message : String(e)))
  }
  useEffect(load, [])

  if (!cfg) {
    return loadError ? (
      <div className="load-notice" role="alert">
        <p className="form-error">{ai.loadError(loadError)}</p>
        <button type="button" onClick={load}>
          {ai.retry}
        </button>
      </div>
    ) : (
      <p className="settings-hint">{ai.loading}</p>
    )
  }

  const stored = settings.find((s) => s.id === 'llm.mode')?.stored
  const originText =
    cfg.source === 'configured' ? (stored?.project !== undefined ? ai.origin.project : ai.origin.host) : ai.origin[cfg.source]
  const sel = path ?? pathOf(cfg)
  const scopeName = scope === 'host' ? t.settings.hostGlobal : t.settings.scopes[scope]

  // run is one panel action: busy-locked (aria-disabled, so the pressed button
  // keeps the focus), and a failure stays visible as an alert.
  const run = async (action: () => Promise<void>) => {
    if (busy) return
    setBusy(true)
    setWriteError(null)
    setNotice('')
    try {
      await action()
    } catch (e) {
      setWriteError(ai.writeFailed(e instanceof Error ? e.message : String(e)))
    } finally {
      setBusy(false)
    }
  }
  const write = (entries: [string, string | null][]) =>
    run(async () => {
      let view: SettingsView | null = null
      for (const [id, value] of entries) view = value === null ? await unsetSetting(id, scope) : await putSetting(id, scope, value)
      if (view) onWritten(view)
      setCfg(await fetchLLMConfig())
      setNotice(ai.saved)
    })
  const test = (name: string, mode: LLMMode) =>
    run(async () => {
      setNotice(ai.testing)
      const r = await testLLM(mode)
      setNotice(r.ok ? ai.testOk(name) : ai.testFail(name, r.error ?? ''))
    })

  const list = sel === 'api' || sel === 'gateway' ? PROVIDERS[sel] : []
  const prov = list.includes(provider) ? provider : list.includes(cfg.provider) ? cfg.provider : (list[0] ?? '')
  const modelValue = model ?? cfg.model
  const baseValue = baseURL ?? cfg.base_url
  const key = keys[prov] ?? (prov === cfg.provider ? cfg.key : undefined)
  const catalog = (cfg.models ?? []).filter((m) => m.provider === 'anthropic' && m.visibility !== 'delisted')
  const title = (p: PathId) => ai.paths[p].title

  const keyState = !key?.set ? ai.keyNone : key.source === 'env' ? ai.keyEnv(key.env) : key.last4 ? ai.keyLast4(key.last4) : ai.keySet
  const saveKey = () =>
    run(async () => {
      setKeys({ ...keys, [prov]: await putLLMKey(prov, keyInput) })
      setKeyInput('')
    })
  const removeKey = () => run(async () => setKeys({ ...keys, [prov]: await deleteLLMKey(prov) }))

  return (
    <section className="ai-panel" aria-label={t.settings.groups.ai.title}>
      <p className="ai-inuse">
        <span className="ai-inuse-label">{ai.inUse}</span> <strong>{describeLLM(cfg, ai)}</strong>{' '}
        <span className="ai-origin">{originText}</span>
      </p>
      <p className="settings-hint">{ai.scopeNote(scopeName)}</p>
      <div className="ai-paths" role="radiogroup" aria-label={ai.pathsLegend}>
        {PATHS.map((p) => (
          <div key={p} className={p === sel ? 'ai-path selected' : 'ai-path'}>
            <label className="ai-path-head">
              <input
                type="radio"
                name={`${uid}-path`}
                value={p}
                checked={p === sel}
                aria-describedby={`${uid}-${p}`}
                onChange={() => setPath(p)}
              />
              <span>{title(p)}</span>
            </label>
            <p id={`${uid}-${p}`} className="settings-hint">
              {ai.paths[p].hint}
            </p>
            {p === sel && p === 'subscription' && (
              <ul className="ai-clis">
                {cfg.clis.map((c) => {
                  const name = ai.clis[c.name] ?? c.name
                  const lock = busy || !c.found || undefined
                  return (
                    <li key={c.name}>
                      <span className="ai-cli-name">{name}</span>
                      <span className="badge">{c.found ? ai.installed : ai.notFound}</span>
                      <button type="button" aria-label={ai.useNamed(name)} aria-disabled={lock} onClick={lock ? undefined : () => write([['llm.mode', c.mode]])}>
                        {ai.use}
                      </button>
                      <button type="button" aria-label={ai.testNamed(name)} aria-disabled={lock} onClick={lock ? undefined : () => test(name, c.mode)}>
                        {ai.test}
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
            {p === sel && p === 'delegation' && (
              <div className="ai-fields">
                <button type="button" aria-label={ai.useNamed(title(p))} aria-disabled={busy || undefined} onClick={busy ? undefined : () => write([['llm.mode', 'delegation']])}>
                  {ai.use}
                </button>
              </div>
            )}
            {p === sel && list.length > 0 && (
              <div className="ai-fields">
                <label>
                  <span>{p === 'api' ? ai.provider : ai.gatewayLabel}</span>
                  <select className="settings-select" value={prov} onChange={(e) => setProvider(e.target.value)}>
                    {list.map((x) => (
                      <option key={x} value={x}>
                        {ai.providers[x] ?? x}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="ai-key">
                  <label>
                    <span>{ai.key}</span>
                    <input
                      type="password"
                      className="settings-text"
                      autoComplete="off"
                      spellCheck={false}
                      value={keyInput}
                      aria-describedby={`${uid}-keyhost ${uid}-keystate`}
                      onChange={(e) => setKeyInput(e.target.value)}
                    />
                  </label>
                  <button type="button" aria-disabled={busy || keyInput.trim() === '' || undefined} onClick={busy || keyInput.trim() === '' ? undefined : saveKey}>
                    {ai.keySave}
                  </button>
                  {key?.set && key.source !== 'env' && (
                    <button type="button" aria-disabled={busy || undefined} onClick={busy ? undefined : removeKey}>
                      {ai.keyRemove}
                    </button>
                  )}
                  <p id={`${uid}-keystate`} className="ai-keystate">
                    {keyState}
                  </p>
                  {key?.problem && <p className="form-error">{ai.keyProblem(key.problem)}</p>}
                  <p id={`${uid}-keyhost`} className="settings-hint">
                    {ai.keyHost}
                  </p>
                </div>
                <label>
                  <span>{ai.model}</span>
                  {prov === 'anthropic' ? (
                    <select className="settings-select" value={modelValue} onChange={(e) => setModel(e.target.value)}>
                      <option value="">{ai.modelCatalog}</option>
                      {modelValue !== '' && !catalog.some((m) => m.id === modelValue) && <option value={modelValue}>{modelValue}</option>}
                      {catalog.map((m) => (
                        <option key={m.id} value={m.id}>
                          {m.label}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <input type="text" className="settings-text" value={modelValue} placeholder={ai.modelDefault} onChange={(e) => setModel(e.target.value)} />
                  )}
                </label>
                {prov === 'compatible' && (
                  <label>
                    <span>{ai.baseUrl}</span>
                    <input type="text" className="settings-text" value={baseValue} placeholder="http://localhost:11434/v1" onChange={(e) => setBaseURL(e.target.value)} />
                  </label>
                )}
                <div className="ai-actions">
                  <button
                    type="button"
                    aria-label={ai.useNamed(title(p))}
                    aria-disabled={busy || undefined}
                    onClick={
                      busy
                        ? undefined
                        : () =>
                            write([
                              ['llm.mode', 'byo-key'],
                              ['llm.provider', prov],
                              ['llm.model', modelValue.trim() || null],
                              ['llm.base_url', prov === 'compatible' ? baseValue.trim() || null : null],
                            ])
                    }
                  >
                    {ai.use}
                  </button>
                  <button type="button" aria-label={ai.testNamed(title(p))} aria-disabled={busy || undefined} onClick={busy ? undefined : () => test(title(p), 'byo-key')}>
                    {ai.test}
                  </button>
                  <span className="settings-hint">{ai.testHint}</span>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
      <p className="ai-notice" role="status" aria-live="polite">
        {notice}
      </p>
      {writeError && (
        <p role="alert" className="form-error">
          {writeError}
        </p>
      )}
    </section>
  )
}
