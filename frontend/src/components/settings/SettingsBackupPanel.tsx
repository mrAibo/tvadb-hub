import { useState } from 'react'
import {
  IconDownload as Download,
  IconFileUpload as FileUpload,
  IconShieldCheck as ShieldCheck,
} from '@tabler/icons-react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { selectFile, selectSaveFile } from '@/services/binaryService'
import { exportSettings, importSettings } from '@/services/settingsService'
import { useSettingsStore } from '@/stores/useSettingsStore'
import { useUIStore } from '@/stores/useUIStore'

export function SettingsBackupPanel() {
  const queryClient = useQueryClient()
  const setAppConfig = useSettingsStore((state) => state.setAppConfig)
  const hydratePreferencesDraft = useSettingsStore((state) => state.hydratePreferencesDraft)
  const setTheme = useUIStore((state) => state.setTheme)
  const [busy, setBusy] = useState<'export' | 'import' | null>(null)

  async function handleExport() {
    setBusy('export')
    try {
      const path = await selectSaveFile('tvadb-hub-settings.json')
      if (!path) return
      await exportSettings(path)
      toast.success('Settings backup exported', { description: path })
    } catch (error) {
      toast.error('Settings export failed', {
        description: error instanceof Error ? error.message : String(error),
      })
    } finally {
      setBusy(null)
    }
  }

  async function handleImport() {
    setBusy('import')
    try {
      const path = await selectFile()
      if (!path) return
      const config = await importSettings(path)
      setAppConfig(config)
      hydratePreferencesDraft(config)
      setTheme(config.theme === 'light' ? 'light' : 'dark')
      await queryClient.invalidateQueries()
      toast.success('Settings backup imported', {
        description: 'Preferences, tool paths and remembered devices were restored.',
      })
    } catch (error) {
      toast.error('Settings import failed', {
        description: error instanceof Error ? error.message : String(error),
      })
    } finally {
      setBusy(null)
    }
  }

  return (
    <Card className="rounded-2xl border border-border/50 bg-card/60 shadow-[var(--shadow-card)]">
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm font-semibold">
          <ShieldCheck className="h-4 w-4 text-muted-foreground" />
          Settings backup
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3 px-5 pb-5">
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          Export a portable JSON backup of TVADB Hub preferences, binary paths, Scrcpy presets,
          nicknames, remembered wireless devices and window state. Machine-specific paths remain
          visible after import and can be corrected from Tool locations if they do not exist on
          the new computer.
        </p>
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="outline"
            className="h-8 gap-1.5 text-xs"
            disabled={busy !== null}
            onClick={() => void handleExport()}
          >
            <Download className="h-3.5 w-3.5" />
            Export JSON backup
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="h-8 gap-1.5 text-xs"
            disabled={busy !== null}
            onClick={() => void handleImport()}
          >
            <FileUpload className="h-3.5 w-3.5" />
            Import JSON backup
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
