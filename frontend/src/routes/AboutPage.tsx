import { useEffect, useState } from 'react'
import {
  IconBrandGithub,
  IconCpu,
  IconDeviceTv,
  IconExternalLink,
  IconInfoCircle,
  IconLicense,
} from '@tabler/icons-react'
import { getAppInfo } from '@/services/aboutService'
import type { AppInfo } from '@/lib/types'

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-6 border-b border-border/40 py-2.5 last:border-b-0">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="text-right text-sm font-medium text-foreground">{value || '—'}</span>
    </div>
  )
}

export default function AboutPage() {
  const [info, setInfo] = useState<AppInfo | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    getAppInfo()
      .then((value) => {
        if (!cancelled) setInfo(value)
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err))
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
      <div className="flex items-center gap-3">
        <IconInfoCircle className="h-6 w-6 text-primary" />
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">About TVADB Hub</h1>
          <p className="text-sm text-muted-foreground">
            Android TV / Google TV management over modern Wireless ADB.
          </p>
        </div>
      </div>

      <section className="grid gap-6 lg:grid-cols-[280px_1fr]">
        <div className="flex flex-col items-center justify-center rounded-2xl border border-border/60 bg-card/60 p-8 text-center shadow-sm">
          <img src="/logo.png" alt="TVADB Hub" className="h-32 w-32 rounded-[28%] object-contain shadow-lg" />
          <h2 className="mt-5 text-2xl font-semibold">TVADB Hub</h2>
          <div className="mt-2 rounded-full border border-primary/25 bg-primary/10 px-3 py-1 text-xs font-medium text-primary">
            v{info?.version ?? '0.1.0'}
          </div>
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground">
            Desktop tooling for pairing, reconnecting, managing apps, remote control, screenshots, Scrcpy,
            Logcat, shell, files and diagnostics on Android TV and Google TV.
          </p>
        </div>

        <div className="rounded-2xl border border-border/60 bg-card/60 p-6 shadow-sm">
          <div className="mb-4 flex items-center gap-2">
            <IconCpu className="h-5 w-5 text-primary" />
            <h3 className="font-semibold">Build & runtime</h3>
          </div>
          <InfoRow label="Version" value={info?.version ?? '0.1.0'} />
          <InfoRow label="Platform" value={info ? `${info.os}/${info.arch}` : 'Loading…'} />
          <InfoRow label="Go" value={info?.goVersion ?? 'Loading…'} />
          <InfoRow label="Wails" value={info?.wailsVersion ?? 'Loading…'} />
          {error && (
            <p className="mt-4 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
              Runtime metadata could not be loaded: {error}
            </p>
          )}
        </div>
      </section>

      <section className="grid gap-4 md:grid-cols-3">
        <a
          href={info?.repository ?? 'https://github.com/mrAibo/tvadb-hub'}
          target="_blank"
          rel="noreferrer"
          className="group rounded-2xl border border-border/60 bg-card/60 p-5 transition-colors hover:border-primary/40 hover:bg-primary/5"
        >
          <div className="flex items-center justify-between">
            <IconBrandGithub className="h-6 w-6" />
            <IconExternalLink className="h-4 w-4 text-muted-foreground transition-colors group-hover:text-primary" />
          </div>
          <div className="mt-4 font-medium">Project repository</div>
          <div className="mt-1 text-sm text-muted-foreground">Source, issues, releases and documentation.</div>
        </a>

        <div className="rounded-2xl border border-border/60 bg-card/60 p-5">
          <IconLicense className="h-6 w-6" />
          <div className="mt-4 font-medium">License</div>
          <div className="mt-1 text-sm text-muted-foreground">{info?.license ?? 'MIT'}</div>
        </div>

        <a
          href={info?.upstreamUrl ?? 'https://github.com/Drenzzz/ADBKit'}
          target="_blank"
          rel="noreferrer"
          className="group rounded-2xl border border-border/60 bg-card/60 p-5 transition-colors hover:border-primary/40 hover:bg-primary/5"
        >
          <div className="flex items-center justify-between">
            <IconDeviceTv className="h-6 w-6" />
            <IconExternalLink className="h-4 w-4 text-muted-foreground transition-colors group-hover:text-primary" />
          </div>
          <div className="mt-4 font-medium">Upstream attribution</div>
          <div className="mt-1 text-sm text-muted-foreground">{info?.upstreamName ?? 'ADBKit v2.0.0'} · MIT</div>
        </a>
      </section>

      <p className="pb-4 text-center text-xs text-muted-foreground">
        TVADB Hub is an independent open-source project and is not affiliated with Google.
      </p>
    </div>
  )
}
