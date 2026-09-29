export interface TVShellShortcut {
  id: string
  label: string
  description: string
  command: string
}

export const TV_SHELL_SHORTCUTS: TVShellShortcut[] = [
  {
    id: 'device',
    label: 'TV identity',
    description: 'Model, Android version and security patch',
    command:
      'getprop ro.product.model; getprop ro.build.version.release; getprop ro.build.version.security_patch',
  },
  {
    id: 'display',
    label: 'Display',
    description: 'Current resolution and density',
    command: 'wm size; wm density',
  },
  {
    id: 'network',
    label: 'Wi-Fi network',
    description: 'Addresses and link state for wlan0',
    command: 'ip addr show wlan0',
  },
  {
    id: 'storage',
    label: 'Storage',
    description: 'Free space for data and shared storage',
    command: 'df -h /data /sdcard',
  },
  {
    id: 'apps',
    label: 'User apps',
    description: 'First 50 third-party packages',
    command: 'pm list packages -3 | head -50',
  },
  {
    id: 'uptime',
    label: 'Uptime',
    description: 'System uptime and load',
    command: 'uptime',
  },
]
