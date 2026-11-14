import { useState, type FormEvent } from 'react'
import { reportBug } from './api'
import { useT } from './i18n'

// "Reportar correção" (T-0006): título + os três campos do relato. O servidor
// cria a B- já aprovada (o relato humano é o approve), pronta para iniciar.
export default function ReportBug() {
  const [title, setTitle] = useState('')
  const [context, setContext] = useState('')
  const [steps, setSteps] = useState('')
  const [expected, setExpected] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const { t } = useT()

  const ready = title.trim() !== '' && context.trim() !== ''

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!ready) return
    setBusy(true)
    setError(null)
    void reportBug(title.trim(), context, steps, expected).then((res) => {
      setBusy(false)
      if (res.ok) {
        window.location.hash = `#/f/${encodeURIComponent(res.feature.id)}`
      } else {
        setError(res.error)
      }
    })
  }

  return (
    <main className="page">
      <a className="back" href="#/manage">{t.newFeature.backToManage}</a>
      <h1>{t.reportBug.title}</h1>
      <form className="new-feature-form" onSubmit={submit}>
        <label>
          {t.reportBug.fieldTitle}
          <input value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
        </label>
        <label>
          {t.reportBug.fieldContext}
          <textarea rows={3} value={context} onChange={(e) => setContext(e.target.value)} />
        </label>
        <label>
          {t.reportBug.fieldSteps}
          <textarea rows={4} value={steps} onChange={(e) => setSteps(e.target.value)} />
        </label>
        <label>
          {t.reportBug.fieldExpected}
          <textarea rows={3} value={expected} onChange={(e) => setExpected(e.target.value)} />
        </label>
        <button type="submit" disabled={busy || !ready}>
          {t.reportBug.submit}
        </button>
        {error && <p className="form-error">{error}</p>}
      </form>
      <p className="muted">{t.reportBug.hint}</p>
    </main>
  )
}
