import { useState, useEffect } from 'react'
import { IconDeviceSdCard } from '@tabler/icons-react'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { listSdCardsForDevice } from '@/services/fileService'
import { useDeviceStore } from '@/stores/useDeviceStore'
import type { SdCard } from '@/lib/types'

interface SdCardPickerProps {
  onSelect: (mountPoint: string) => void
  disabled?: boolean
  /**
   * Confirmed device target. When the caller passes it, the volumes are read for
   * exactly that device; when it is omitted the picker reads the confirmed selection
   * from the shared device store. It never issues a serial-less request.
   */
  serial?: string
}

export function SdCardPicker({ onSelect, disabled, serial }: SdCardPickerProps) {
  const [cards, setCards] = useState<SdCard[]>([])
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) return
    const target = serial ?? useDeviceStore.getState().activeSerial
    if (!target) {
      // No confirmed device: refuse locally instead of asking the backend without one.
      setCards([])
      return
    }
    setLoading(true)
    listSdCardsForDevice(target)
      .then((next) => {
        // A reply for a device the user has already left must not render.
        if (useDeviceStore.getState().activeSerial === target) setCards(next)
      })
      .catch(() => setCards([]))
      .finally(() => setLoading(false))
  }, [open, serial])

  const externalCards = cards.filter((c) => c.isExternal)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        disabled={disabled}
        className="inline-flex items-center justify-center gap-1.5 rounded-full border border-border bg-muted/50 px-3 py-1.5 text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-muted/70 transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
      >
        <IconDeviceSdCard className="h-3.5 w-3.5" />
        Storage
      </PopoverTrigger>
      <PopoverContent align="end" className="w-64 p-2">
        {loading && (
          <p className="text-xs text-muted-foreground px-2 py-1">Detecting volumes…</p>
        )}
        {!loading && externalCards.length === 0 && cards.length === 0 && (
          <p className="text-xs text-muted-foreground px-2 py-1">
            No storage volumes detected.
          </p>
        )}
        {!loading && externalCards.length > 0 && (
          <div className="space-y-0.5">
            <p className="text-xs font-semibold text-muted-foreground px-2 py-1">
              External SD cards
            </p>
            {externalCards.map((card) => (
              <button
                key={card.id}
                onClick={() => {
                  onSelect(card.mountPoint)
                  setOpen(false)
                }}
                className="w-full flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent text-left transition-colors cursor-pointer"
              >
                <IconDeviceSdCard className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0">
                  <span className="block truncate font-medium">{card.description}</span>
                  <span className="block truncate text-xs text-muted-foreground font-mono">
                    {card.mountPoint}
                  </span>
                </span>
              </button>
            ))}
          </div>
        )}
        {!loading && cards.length > 0 && externalCards.length === 0 && (
          <div className="space-y-0.5">
            <p className="text-xs font-semibold text-muted-foreground px-2 py-1">
              Internal storage
            </p>
            {cards.map((card) => (
              <button
                key={card.id}
                onClick={() => {
                  onSelect(card.mountPoint)
                  setOpen(false)
                }}
                className="w-full flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent text-left transition-colors cursor-pointer"
              >
                <IconDeviceSdCard className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0">
                  <span className="block truncate font-medium">{card.description}</span>
                  <span className="block truncate text-xs text-muted-foreground font-mono">
                    {card.mountPoint}
                  </span>
                </span>
              </button>
            ))}
          </div>
        )}
      </PopoverContent>
    </Popover>
  )
}
